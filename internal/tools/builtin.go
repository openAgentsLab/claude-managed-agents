package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"

	"forge/internal/skill"
	"forge/internal/skill/bundled"
	exectools "forge/internal/tools/exec"
	fstools "forge/internal/tools/fs"
	gittools "forge/internal/tools/git"
	mcptools "forge/internal/tools/mcp"
	memorytools "forge/internal/tools/memory"
	skilltool "forge/internal/tools/skill"
)

func init() {
	// fs, git, exec: sandboxed so that in docker mode they route through the
	// container via tool-exec.  In local mode they still run in-process.
	RegisterSandboxedSource(func(ctx context.Context) (ToolRegistry, func()) {
		reg, err := fstools.NewFsRegistry(ctx)
		if err != nil {
			return nil, nil
		}
		return reg, nil
	})

	RegisterSandboxedSource(func(ctx context.Context) (ToolRegistry, func()) {
		reg, err := gittools.NewGitRegistry(ctx)
		if err != nil {
			return nil, nil
		}
		return reg, nil
	})

	RegisterSandboxedSource(func(ctx context.Context) (ToolRegistry, func()) {
		reg, err := exectools.NewExecRegistry(ctx)
		if err != nil {
			return nil, nil
		}
		return reg, nil
	})

	RegisterSource(func(_ context.Context) (ToolRegistry, func()) {
		ts, err := memorytools.NewTools()
		if err != nil {
			return nil, nil
		}
		return Static(ts), nil
	})
	RegisterReadOnly("memory_list", "memory_read", "memory_search")

	// Skill: bundled inline skills only. User-configured skills are injected
	// per-session via the session brain (orchestration/session_brain.go).
	RegisterSource(func(ctx context.Context) (ToolRegistry, func()) {
		reg := skill.NewRegistry()
		bundled.Init(reg)
		return Static([]tool.BaseTool{skilltool.New(reg)}), nil
	})

	// MCP: platform-level servers from forge.yaml tools.mcp_servers.
	// Configs are injected into ctx by hands.BuildSandboxLayer via mcptools.WithConfig.
	// User-configured MCP servers are handled per-session by the session brain.
	RegisterSource(func(ctx context.Context) (ToolRegistry, func()) {
		reg, cleanup, err := mcptools.NewRegistry(ctx)
		if err != nil || reg == nil {
			return nil, nil
		}
		return reg, cleanup
	})

	// Declare which tools are read-only so the permission engine can allow them
	// automatically in plan mode, without the permission package knowing tool names.
	RegisterReadOnly(
		"read_file", "glob", "grep_file", "list_dir",
		"git_status", "git_log", "git_diff", "git_show", "git_blame",
	)
}


