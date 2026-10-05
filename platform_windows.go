//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

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

// attachConsole connects a windowsgui build to the console it was started
// from, so the command line version can print. Output that is already
// redirected to a file or pipe is left as it is.
func attachConsole() {
	if hasStdout() {
		return
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	const attachParentProcess = ^uintptr(0)
	if r, _, _ := kernel32.NewProc("AttachConsole").Call(attachParentProcess); r == 0 {
		return
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = f
		os.Stderr = f
	}
}

func hasStdout() bool {
	h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil || h == syscall.InvalidHandle || h == 0 {
		return false
	}
	t, err := syscall.GetFileType(h)
	return err == nil && t != 0 // FILE_TYPE_UNKNOWN
}

func hideFile(path string) {
	if p, err := syscall.UTF16PtrFromString(path); err == nil {
		if attrs, err := syscall.GetFileAttributes(p); err == nil {
			syscall.SetFileAttributes(p, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
		}
	}
}
