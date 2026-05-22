package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// initialSync does a one-shot List of existing Pods to populate the cache
// before the Watch starts, so existing sandboxes are visible immediately on
// worker startup.
func (p *K8sWatchPool) initialSync(ctx context.Context) {
	pods, err := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: sandboxLabelSelector,
	})
	if err != nil {
		slog.WarnContext(ctx, "k8s watch pool: initial pod list failed", "error", err)
		return
	}
	p.mu.Lock()
	for _, pod := range pods.Items {
		key := pod.Labels[labelSandboxKey]
		if key == "" {
			continue
		}
		p.cache[key] = &k8sSandboxEntry{
			token:      tokenFromPod(&pod),
			serviceURL: p.serviceURL(key),
			ready:      isPodReady(&pod),
		}
	}
	p.mu.Unlock()
}

func (p *K8sWatchPool) runPodWatch(ctx context.Context) {
	for ctx.Err() == nil {
		if err := p.podWatch(ctx); err != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "k8s watch pool: pod watch error, reconnecting",
				"error", err, "delay", watchReconnectDelay)
			select {
			case <-ctx.Done():
			case <-time.After(watchReconnectDelay):
			}
		}
	}
}

func (p *K8sWatchPool) podWatch(ctx context.Context) error {
	list, err := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: sandboxLabelSelector,
	})
	if err != nil {
		return err
	}
	w, err := p.client.CoreV1().Pods(p.namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector:   sandboxLabelSelector,
		ResourceVersion: list.ResourceVersion,
	})
	if err != nil {
		return err
	}
	defer w.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.ResultChan():
			if !ok {
				return fmt.Errorf("pod watch channel closed")
			}
			pod, ok := ev.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			key := pod.Labels[labelSandboxKey]
			if key == "" {
				continue
			}
			p.mu.Lock()
			switch ev.Type {
			case watch.Added, watch.Modified:
				phase := pod.Status.Phase
				if phase == corev1.PodFailed || phase == corev1.PodSucceeded {
					// Pod terminated. Delete the K8s object so the next Acquire
					// can create a fresh pod (Create would return AlreadyExists
					// otherwise, and waitReady would time out).
					delete(p.cache, key)
					go func(k string) {
						if err := p.deletePod(context.Background(), k); err != nil {
							slog.Warn("k8s watch: delete terminated pod",
								"key", k, "error", err)
						}
					}(key)
				} else if e, exists := p.cache[key]; exists {
					e.ready = isPodReady(pod)
				} else {
					p.cache[key] = &k8sSandboxEntry{
						token:      tokenFromPod(pod),
						serviceURL: p.serviceURL(key),
						ready:      isPodReady(pod),
					}
				}
			case watch.Deleted:
				// Pod deleted (idle cleanup or ReleaseSession); remove from cache.
				// With RestartPolicy:Never, deleted pods are not recreated automatically.
				delete(p.cache, key)
			}
			p.mu.Unlock()
		}
	}
}

func tokenFromPod(pod *corev1.Pod) string {
	for _, c := range pod.Spec.Containers {
		for _, env := range c.Env {
			if env.Name == "TOOL_SERVER_TOKEN" {
				return env.Value
			}
		}
	}
	return ""
}

func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	// All init containers must have completed successfully.
	for _, cs := range pod.Status.InitContainerStatuses {
		if !cs.Ready {
			return false
		}
	}
	// All main containers must be ready (readiness probe passed).
	for _, cs := range pod.Status.ContainerStatuses {
		if !cs.Ready {
			return false
		}
	}
	return len(pod.Status.ContainerStatuses) > 0
}
