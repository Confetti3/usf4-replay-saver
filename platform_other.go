//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
)

func steamRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(home, ".steam", "debian-installation"),
	}
}

func defaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "USF4 Replays"
	}
	return filepath.Join(home, "USF4 Replays")
}

// gameRunning looks for SSFIV.exe in any process command line, which covers
// Proton and Wine.
func gameRunning() (bool, error) {
	procs, err := filepath.Glob("/proc/[0-9]*/cmdline")
	if err != nil {
		return false, err
	}
	for _, path := range procs {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(data)), "ssfiv.exe") {
			return true, nil
		}
	}
	return false, nil
}

const guiAvailable = false

func runGUI(minimized bool) int { return 1 }

func attachConsole() {}
