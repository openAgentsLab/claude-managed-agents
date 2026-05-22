package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"forge/internal/config"
	"forge/internal/hands"
	"forge/internal/resources"
)

func (p *K8sWatchPool) createSandbox(ctx context.Context, key, token string, env resources.Environment, quota hands.ResourceQuota) error {
	if err := p.ensurePVC(ctx, workspacePVCName(key)); err != nil {
		return err
	}
	if err := p.ensureService(ctx, key); err != nil {
		return err
	}
	if env.Networking.Mode == resources.NetworkingLimited {
		if err := p.ensureNetworkPolicy(ctx, key, env.Networking.AllowedHosts); err != nil {
			return err
		}
	}
	return p.ensurePod(ctx, key, token, env, quota)
}

func (p *K8sWatchPool) ensurePVC(ctx context.Context, pvcName string) error {
	_, err := p.client.CoreV1().PersistentVolumeClaims(p.namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !k8serrors.IsNotFound(err) {
		return fmt.Errorf("get PVC %q: %w", pvcName, err)
	}
	qty, err := resource.ParseQuantity(p.workspaceSize)
	if err != nil {
		return fmt.Errorf("parse workspace_size %q: %w", p.workspaceSize, err)
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: p.namespace,
			Labels:    map[string]string{labelApp: labelAppVal},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: qty},
			},
		},
	}
	if p.storageClass != "" {
		pvc.Spec.StorageClassName = &p.storageClass
	}
	_, err = p.client.CoreV1().PersistentVolumeClaims(p.namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (p *K8sWatchPool) ensureService(ctx context.Context, key string) error {
	svcName := sandboxName(key)
	_, err := p.client.CoreV1().Services(p.namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !k8serrors.IsNotFound(err) {
		return fmt.Errorf("get service %q: %w", svcName, err)
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: p.namespace,
			Labels:    map[string]string{labelApp: labelAppVal, labelSandboxKey: key},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{labelApp: labelAppVal, labelSandboxKey: key},
			Ports: []corev1.ServicePort{{
				Port:       int32(toolServerPort),
				TargetPort: intstr.FromInt32(int32(toolServerPort)),
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	_, err = p.client.CoreV1().Services(p.namespace).Create(ctx, svc, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (p *K8sWatchPool) ensurePod(ctx context.Context, key, token string, env resources.Environment, quota hands.ResourceQuota) error {
	pod := p.buildPod(key, token, env, quota)
	_, err := p.client.CoreV1().Pods(p.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (p *K8sWatchPool) buildPod(key, token string, env resources.Environment, quota hands.ResourceQuota) *corev1.Pod {
	labels := map[string]string{labelApp: labelAppVal, labelSandboxKey: key}

	limits := corev1.ResourceList{}
	memBytes := quota.MemoryBytes
	nanoCPUs := quota.NanoCPUs
	if memBytes > 0 {
		limits[corev1.ResourceMemory] = *resource.NewQuantity(memBytes, resource.BinarySI)
	}
	if nanoCPUs > 0 {
		limits[corev1.ResourceCPU] = *resource.NewMilliQuantity(nanoCPUs/1_000_000, resource.DecimalSI)
	}

	// Build container env: TOOL_SERVER_TOKEN first, then user-defined vars.
	containerEnv := []corev1.EnvVar{{Name: "TOOL_SERVER_TOKEN", Value: token}}
	for k, v := range env.Env {
		if reservedEnvVar(k) {
			continue
		}
		containerEnv = append(containerEnv, corev1.EnvVar{Name: k, Value: v})
	}

	// Wrap in sh -c so env.sh (written by sandbox-init) is sourced before the
	// tool-server starts, making installed packages visible to every tool call.
	envSource := ". " + config.ContainerWorkspaceRoot + "/.forge-env/env.sh 2>/dev/null || true"
	serverCmd := fmt.Sprintf("{ %s; }; exec /forge tool-server --addr :%d", envSource, toolServerPort)

	mainContainer := corev1.Container{
		Name:    "tool-server",
		Image:   p.image,
		Command: []string{"sh", "-c", serverCmd},
		Env:     containerEnv,
		Ports: []corev1.ContainerPort{
			{ContainerPort: int32(toolServerPort), Protocol: corev1.ProtocolTCP},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt32(int32(toolServerPort)),
				},
			},
			InitialDelaySeconds: 2,
			PeriodSeconds:       2,
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: config.ContainerWorkspaceRoot},
		},
	}
	if len(limits) > 0 {
		mainContainer.Resources = corev1.ResourceRequirements{Limits: limits}
	}

	volumes := []corev1.Volume{{
		Name: "workspace",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: workspacePVCName(key),
			},
		},
	}}
	podSpec := corev1.PodSpec{
		Containers:    []corev1.Container{mainContainer},
		RestartPolicy: corev1.RestartPolicyNever,
		Volumes:       volumes,
	}
	if p.serviceAccount != "" {
		podSpec.ServiceAccountName = p.serviceAccount
	}

	// Init container installs declared packages once; subsequent pod rebuilds
	// skip installation because forge sandbox-init checks a hash in the PVC.
	if !env.Packages.IsEmpty() {
		packagesJSON, _ := json.Marshal(env.Packages)
		initContainer := corev1.Container{
			Name:  "forge-init",
			Image: p.image,
			Command: []string{
				"/forge", "sandbox-init",
				"--workspace", config.ContainerWorkspaceRoot,
			},
			Env: []corev1.EnvVar{
				{Name: "FORGE_PACKAGES_SPEC", Value: string(packagesJSON)},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "workspace", MountPath: config.ContainerWorkspaceRoot},
			},
		}
		podSpec.InitContainers = []corev1.Container{initContainer}
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sandboxName(key),
			Namespace: p.namespace,
			Labels:    labels,
		},
		Spec: podSpec,
	}
}
