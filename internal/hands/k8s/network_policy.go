package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1 "k8s.io/api/core/v1"
)

func networkPolicyName(key string) string { return "forge-np-" + key }

// ensureNetworkPolicy creates a NetworkPolicy for the pod keyed by key.
// The policy allows:
//   - all ingress (orchestration must reach the tool-server)
//   - DNS egress (UDP+TCP port 53)
//   - egress to each host in allowedHosts resolved to CIDR
//
// All other egress is denied.
func (p *K8sWatchPool) ensureNetworkPolicy(ctx context.Context, key string, allowedHosts []string) error {
	name := networkPolicyName(key)
	_, err := p.client.NetworkingV1().NetworkPolicies(p.namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !k8serrors.IsNotFound(err) {
		return fmt.Errorf("get network policy %q: %w", name, err)
	}

	dnsPort53UDP := intstr.FromInt32(53)
	dnsPort53TCP := intstr.FromInt32(53)
	protoUDP := corev1.ProtocolUDP
	protoTCP := corev1.ProtocolTCP

	// DNS egress is always allowed so name resolution works.
	egressRules := []networkingv1.NetworkPolicyEgressRule{
		{
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: &protoUDP, Port: &dnsPort53UDP},
				{Protocol: &protoTCP, Port: &dnsPort53TCP},
			},
		},
	}

	// Resolve each allowed host to its IP(s) and add CIDR egress rules.
	for _, host := range allowedHosts {
		addrs, err := net.LookupHost(host)
		if err != nil {
			slog.WarnContext(ctx, "k8s network policy: failed to resolve allowed host; skipping",
				"host", host, "error", err)
			continue
		}
		var cidrs []networkingv1.IPBlock
		for _, addr := range addrs {
			ip := net.ParseIP(addr)
			if ip == nil {
				continue
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			cidrs = append(cidrs, networkingv1.IPBlock{CIDR: fmt.Sprintf("%s/%d", addr, bits)})
		}
		for _, cidr := range cidrs {
			block := cidr
			egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{{IPBlock: &block}},
			})
		}
	}

	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
			Labels:    map[string]string{labelApp: labelAppVal, labelSandboxKey: key},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{labelSandboxKey: key},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			// Allow all ingress so orchestration can reach the tool-server.
			Ingress: []networkingv1.NetworkPolicyIngressRule{{}},
			Egress:  egressRules,
		},
	}

	_, err = p.client.NetworkingV1().NetworkPolicies(p.namespace).Create(ctx, np, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// deleteNetworkPolicy removes the NetworkPolicy for key if it exists.
func (p *K8sWatchPool) deleteNetworkPolicy(ctx context.Context, key string) {
	name := networkPolicyName(key)
	if err := p.client.NetworkingV1().NetworkPolicies(p.namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		slog.WarnContext(ctx, "k8s release: delete network policy", "key", key, "error", err)
	}
}
