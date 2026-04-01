package shared

import (
	"os"
	"path/filepath"
)

var (
	HomeDir     = getHomeDir()
	ConfigDir   = filepath.Join(HomeDir, ".claude-code-router")
	ProjectsDir = filepath.Join(HomeDir, ".claude", "projects")
	PIDFile     = filepath.Join(HomeDir, ".claude-code-router", ".claude-code-router.pid")
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
