package fs

import (
	"fmt"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
)

// SkillActivator is called after a file tool successfully operates on a path.
// Implementations should check whether any conditional skills match the path,
// promote them to active, and return a <system-reminder> notification string
// listing the newly activated skills for the LLM to see in the tool result.
// Return "" when no skills were newly activated.
// The cwd argument is the workspace root.
type SkillActivator func(filePaths []string, cwd string) string

// SkillDiscoverer is called after a file tool successfully operates on a path.
// Implementations should walk up the directory tree from the path looking for
// .forge/skills/ directories that have not been loaded yet, and load them.
// The cwd argument is the workspace root (upper bound for the walk).
type SkillDiscoverer func(filePaths []string, cwd string) []string

// notifySkillHooks calls discoverer then activator for the given file path.
// It returns any skill-listing notification produced by the activator so that
// callers can append it to the tool's return value — mirroring forge's
// behaviour of injecting a skill_listing system-reminder into the tool results
// immediately when a conditional skill is activated mid-run.
func notifySkillHooks(filePath, cwd string, discoverer SkillDiscoverer, activator SkillActivator) string {
	fp := []string{filePath}
	if discoverer != nil {
		discoverer(fp, cwd)
	}
	if activator != nil {
		return activator(fp, cwd)
	}
	return ""
}

// NewTools returns all workspace filesystem tools.
// discoverer and activator may be nil; when provided they are called after each
// successful read/write/edit operation so dynamic and conditional skills fire.
func NewTools(workspaceRoot string, activator SkillActivator, discoverer SkillDiscoverer) ([]tool.BaseTool, error) {
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}

	list, err := newListDirTool(root)
	if err != nil {
		return nil, err
	}
	read, err := newReadFileTool(root, activator, discoverer)
	if err != nil {
		return nil, err
	}
	edit, err := newFileEditTool(root, activator, discoverer)
	if err != nil {
		return nil, err
	}
	write, err := newWriteFileTool(root, activator, discoverer)
	if err != nil {
		return nil, err
	}
	glob, err := newGlobTool(root)
	if err != nil {
		return nil, err
	}
	grep, err := newGrepFileTool(root)
	if err != nil {
		return nil, err
	}
	delFile, err := newDeleteFileTool(root)
	if err != nil {
		return nil, err
	}
	delDir, err := newDeleteDirTool(root)
	if err != nil {
		return nil, err
	}
	move, err := newMoveFileTool(root)
	if err != nil {
		return nil, err
	}

	return []tool.BaseTool{list, read, edit, write, glob, grep, delFile, delDir, move}, nil
}
