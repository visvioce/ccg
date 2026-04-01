package shared

import (
	"os"
	"path/filepath"
)

var (
	HomeDir            = getHomeDir()
	ConfigDir          = filepath.Join(HomeDir, ".claude-code-router")
	ProjectsDir        = filepath.Join(HomeDir, ".claude", "projects")
	PIDFile            = filepath.Join(HomeDir, ".claude-code-router", ".claude-code-router.pid")
	PluginsDir         = filepath.Join(HomeDir, ".claude-code-router", "plugins")
	PresetsDir         = filepath.Join(HomeDir, ".claude-code-router", "presets")
	ConfigFile         = filepath.Join(HomeDir, ".claude-code-router", "config.json")
	ReferenceCountFile = filepath.Join(os.TempDir(), "claude-code-reference-count.txt")
)

func getHomeDir() string {
	home := os.Getenv("HOME")
	if home != "" {
		return home
	}
	home = os.Getenv("USERPROFILE")
	if home != "" {
		return home
	}
	return "/root"
}
