// Package k8s implements a Kubernetes-backed sandbox pool for forge.
// Each session gets a dedicated Pod (RestartPolicy: Never) + Service + PVC.
// The K8sWatchPool uses the Kubernetes Watch API to track Pod readiness.
// Idle pods are deleted by a background cleanup loop; Service and PVC survive
// until ReleaseSession is called.
//
// Activate by importing this package with a blank identifier:
//
//	import _ "forge/internal/hands/k8s"
package k8s

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"forge/internal/config"
	"forge/internal/hands"
)

const (
	toolServerPort       = 7777
	containerPort        = int32(toolServerPort)
	defaultNamespace     = "default"
	defaultImage         = "forge:latest"
	defaultWorkspaceSize = "1Gi"
	defaultPodIdleTimeout     = 30 * time.Minute
	defaultSessionIdleTimeout = 24 * time.Hour
	defaultReadyTimeout  = 60 * time.Second
	readyPollInterval    = 2 * time.Second

	labelSandboxKey = "forge.io/sandbox-key" // sanitized sessionID
	labelApp        = "app.kubernetes.io/name"
	labelAppVal     = "forge-sandbox"
)

func init() {
	hands.RegisterPool(config.SandboxDriverK8s, func(_ context.Context, cfg config.SandboxConfig, deps hands.PoolDeps) (hands.Pool, error) {
		return newK8sWatchPool(cfg, deps.Sandbox, deps.Resources)
	})
}

// buildClient creates a kubernetes.Clientset, preferring in-cluster config
// then falling back to the kubeconfig path from opts.
func buildClient(opts map[string]string) (kubernetes.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := opts["kubeconfig"]
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("load kubeconfig: %w", err)
		}
	}
	return kubernetes.NewForConfig(cfg)
}

// ── name helpers ──────────────────────────────────────────────────────────────

var nonDNS = regexp.MustCompile(`[^a-z0-9-]`)

// sanitizeName converts an arbitrary string into a k8s-safe lowercase DNS
// label (max 54 chars, alphanumeric + hyphens only, no leading/trailing hyphen).
func sanitizeName(s string) string {
	s = strings.ToLower(s)
	s = nonDNS.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 54 {
		s = s[:54]
	}
	return s
}

func workspacePVCName(key string) string { return "forge-ws-" + key }

// sandboxName returns the shared name used for both the Pod and Service.
func sandboxName(key string) string { return "forge-sb-" + key }

func optOr(opts map[string]string, key, fallback string) string {
	if v := opts[key]; v != "" {
		return v
	}
	return fallback
}

func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// reservedEnvVar reports whether name is a system env var that must not be
// overridden by user-defined environment maps.
func reservedEnvVar(name string) bool {
	switch name {
	case "TOOL_SERVER_TOKEN", "FORGE_PACKAGES_SPEC":
		return true
	}
	return false
}
