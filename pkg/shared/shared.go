package shared

import (
	"os"
	"path/filepath"
)

var (
	HomeDir     = getHomeDir()
	ConfigDir   = filepath.Join(HomeDir, ".ccg")
	ProjectsDir = filepath.Join(HomeDir, ".claude", "projects")
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
