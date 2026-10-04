//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

func steamRoots() []string {
	var roots []string
	for _, key := range []struct {
		root  registry.Key
		path  string
		value string
	}{
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\Valve\Steam`, "InstallPath"},
	} {
		k, err := registry.OpenKey(key.root, key.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		if v, _, err := k.GetStringValue(key.value); err == nil && v != "" {
			roots = append(roots, filepath.Clean(v))
		}
		k.Close()
	}
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
		if base := os.Getenv(env); base != "" {
			roots = append(roots, filepath.Join(base, "Steam"))
		}
	}
	return roots
}

func defaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "USF4 Replays"
	}
	return filepath.Join(home, "Documents", "USF4 Replays")
}

func gameRunning() (bool, error) {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq SSFIV.exe", "/NH", "/FO", "CSV")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(string(out)), "ssfiv.exe"), nil
}

// launchedFromExplorer reports whether the tool owns its console window,
// which is the case when someone double-clicks the exe.
func launchedFromExplorer() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetConsoleProcessList")
	pids := make([]uint32, 4)
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return n == 1
}
