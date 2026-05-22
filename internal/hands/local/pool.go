// Package local implements the LocalSandbox driver for the hands.Sandbox
// interface. Import this package with a blank identifier to register the
// "local" driver:
//
//	import _ "forge/internal/hands/local"
package local

import (
	"context"

	"forge/internal/config"
	"forge/internal/hands"
)

func init() {
	hands.RegisterPool(config.SandboxDriverLocal, func(ctx context.Context, _ config.SandboxConfig, deps hands.PoolDeps) (hands.Pool, error) {
		sb := NewLocalSandbox()
		if err := sb.Provision(ctx, deps.Sandboxed); err != nil {
			return nil, err
		}
		return &LocalPool{sandbox: sb}, nil
	})
}

// LocalPool implements hands.Pool for the local (in-process) driver.
// All users share one LocalSandbox instance; no container lifecycle management
// is needed.
type LocalPool struct {
	sandbox hands.Sandbox
}

func (p *LocalPool) Acquire(_ context.Context, _ string, _ hands.AcquireRequest) (hands.Sandbox, error) {
	return p.sandbox, nil
}

func (p *LocalPool) ReleaseSession(_ context.Context, _ string) error { return nil }

func (p *LocalPool) StartBackground(_ context.Context) {}

func (p *LocalPool) CloseAll() { _ = p.sandbox.Close() }

func (p *LocalPool) Isolated() bool { return false }
