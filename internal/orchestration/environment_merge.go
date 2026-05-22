package orchestration

import (
	"forge/internal/gateway/store"
	"forge/internal/resources"
)

// MergedEnvironment is the fully resolved session configuration produced by
// merging env layers (tenant default → project envId) and attaching the
// project's GitConfig, RefFiles, and inline Env overrides.
//
// Merge rules by field:
//   - Packages:   union across all env layers (no duplicates)
//   - Networking: most restrictive mode wins ("limited" beats "unrestricted");
//                 AllowedHosts are unioned from limited layers only; cleared when final mode is unrestricted
//   - Env:        last-writer-wins — project inline Env overrides envId Env
//   - GitConfig:  taken directly from the project (zero value when no project)
//   - RefFiles:   taken directly from the project (nil when no project)
type MergedEnvironment struct {
	Packages   store.PackageList
	Networking store.NetworkingConfig
	Env        map[string]string
	GitConfig  store.GitConfig
	RefFiles   []store.RefFile
}

// MergeEnvironments merges env-layer records (Packages, Networking, Env).
// Layers are passed lowest-priority first; nil layers are skipped.
// GitConfig and RefFiles are NOT set here — they are attached by resolveSession.
func MergeEnvironments(layers ...*store.Environment) MergedEnvironment {
	var result MergedEnvironment
	result.Networking.Mode = store.NetworkingUnrestricted

	seenPkg := map[string]struct{}{}

	for _, e := range layers {
		if e == nil {
			continue
		}
		for _, p := range e.Packages.Pip {
			if k := "pip:" + p; addIfAbsent(seenPkg, k) {
				result.Packages.Pip = append(result.Packages.Pip, p)
			}
		}
		for _, p := range e.Packages.Npm {
			if k := "npm:" + p; addIfAbsent(seenPkg, k) {
				result.Packages.Npm = append(result.Packages.Npm, p)
			}
		}
		for _, p := range e.Packages.Apt {
			if k := "apt:" + p; addIfAbsent(seenPkg, k) {
				result.Packages.Apt = append(result.Packages.Apt, p)
			}
		}
		for _, p := range e.Packages.Cargo {
			if k := "cargo:" + p; addIfAbsent(seenPkg, k) {
				result.Packages.Cargo = append(result.Packages.Cargo, p)
			}
		}

		if e.Networking.Mode == store.NetworkingLimited {
			result.Networking.Mode = store.NetworkingLimited
			result.Networking.AllowedHosts = unionStrings(result.Networking.AllowedHosts, e.Networking.AllowedHosts)
		}

		// Env: last-writer-wins across env layers.
		for k, v := range e.Env {
			if result.Env == nil {
				result.Env = make(map[string]string)
			}
			result.Env[k] = v
		}
	}

	// If the final mode is unrestricted, AllowedHosts collected from limited layers
	// are irrelevant — clear them so the sandbox gets a clean unrestricted config.
	if result.Networking.Mode != store.NetworkingLimited {
		result.Networking.AllowedHosts = nil
	}

	return result
}

// PackagesToSpec converts a store.PackageList to resources.PackageSpec.
func PackagesToSpec(pl store.PackageList) resources.PackageSpec {
	return resources.PackageSpec{
		Pip:   pl.Pip,
		Npm:   pl.Npm,
		Apt:   pl.Apt,
		Cargo: pl.Cargo,
	}
}

// toEnvironment converts the sandbox-relevant fields of a MergedEnvironment
// (Packages, Networking, Env) into a resources.Environment for injection into
// the sandbox container. GitConfig and RefFiles are handled separately as
// workspace resources, not container environment.
func toEnvironment(m MergedEnvironment) resources.Environment {
	mode := resources.NetworkingUnrestricted
	if m.Networking.Mode == store.NetworkingLimited {
		mode = resources.NetworkingLimited
	}
	return resources.Environment{
		Packages: PackagesToSpec(m.Packages),
		Networking: resources.NetworkConfig{
			Mode:         mode,
			AllowedHosts: m.Networking.AllowedHosts,
		},
		Env: m.Env,
	}
}

// hasAnyPackages reports whether pl specifies at least one package.
func hasAnyPackages(pl store.PackageList) bool {
	return len(pl.Pip)+len(pl.Npm)+len(pl.Apt)+len(pl.Cargo) > 0
}

func addIfAbsent(seen map[string]struct{}, key string) bool {
	if _, ok := seen[key]; ok {
		return false
	}
	seen[key] = struct{}{}
	return true
}

func unionStrings(a, b []string) []string {
	seen := make(map[string]struct{}, len(a))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	for _, s := range b {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
