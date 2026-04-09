package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func installCmd() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}

	binDir := filepath.Join(home, ".local", "bin")
	targetPath := filepath.Join(binDir, "warpspawn")

	// Get the path of the currently running binary
	selfPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine binary path: %v\n", err)
		os.Exit(1)
	}
	selfPath, _ = filepath.EvalSymlinks(selfPath)

	// Check if already installed at target
	if selfPath == targetPath {
		fmt.Println("Warpspawn is already installed at", targetPath)
		ensureDataDirs(home)
		checkPath(binDir)
		return
	}

	// Check if something else exists at target
	if info, err := os.Stat(targetPath); err == nil {
		if info.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: %s is a directory\n", targetPath)
			os.Exit(1)
		}
		fmt.Printf("Existing binary found at %s. Overwrite? [y/N] ", targetPath)
		if !confirm() {
			fmt.Println("Cancelled.")
			return
		}
	}

	// Create bin directory
	if err := os.MkdirAll(binDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create %s: %v\n", binDir, err)
		os.Exit(1)
	}

	// Copy binary
	src, err := os.ReadFile(selfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot read %s: %v\n", selfPath, err)
		os.Exit(1)
	}

	tmpPath := targetPath + ".tmp"
	if err := os.WriteFile(tmpPath, src, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot write %s: %v\n", tmpPath, err)
		os.Remove(tmpPath)
		os.Exit(1)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot install to %s: %v\n", targetPath, err)
		os.Remove(tmpPath)
		os.Exit(1)
	}

	fmt.Printf("Installed to %s\n", targetPath)

	// Create data directories
	ensureDataDirs(home)

	// Check PATH
	checkPath(binDir)

	// Environment checks
	fmt.Println()
	runInstallChecks()

	fmt.Println("\nDone. Run 'warpspawn' from any directory to start.")
}

func runInstallChecks() {
	fmt.Println("Environment checks:")

	// Check git
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Println("  ⚠ git not found — auto-commit (rollback support) will be disabled")
	} else {
		fmt.Println("  ✓ git found")
	}

	// Check Ollama
	resp, err := http.Get("http://localhost:11434/api/version")
	if err != nil {
		fmt.Println("  ⚠ Ollama not detected at localhost:11434 — you'll need a cloud API key (OpenAI or Anthropic)")
	} else {
		resp.Body.Close()
		fmt.Println("  ✓ Ollama detected")
	}

	// Check default port
	ln, err := net.Listen("tcp", "127.0.0.1:9320")
	if err != nil {
		fmt.Println("  ⚠ Port 9320 is in use — Warpspawn will auto-select another port at startup")
	} else {
		ln.Close()
		fmt.Println("  ✓ Port 9320 available")
	}

	// Check disk space
	var stat syscall.Statfs_t
	home, _ := os.UserHomeDir()
	if err := syscall.Statfs(home, &stat); err == nil {
		freeBytes := stat.Bavail * uint64(stat.Bsize)
		freeMB := freeBytes / (1024 * 1024)
		if freeMB < 500 {
			fmt.Printf("  ⚠ Low disk space: %d MB free (recommend 500+ MB)\n", freeMB)
		} else {
			fmt.Printf("  ✓ Disk space: %d MB free\n", freeMB)
		}
	}
}

func uninstallCmd() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}

	binPath := filepath.Join(home, ".local", "bin", "warpspawn")
	configDir := filepath.Join(home, ".config", "warpspawn")
	dataDir := filepath.Join(home, ".local", "share", "warpspawn")

	// Remove binary
	if _, err := os.Stat(binPath); err == nil {
		if err := os.Remove(binPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot remove %s: %v\n", binPath, err)
		} else {
			fmt.Printf("Removed %s\n", binPath)
		}
	} else {
		fmt.Printf("Binary not found at %s (already removed?)\n", binPath)
	}

	// Ask about config and data
	hasConfig := dirExists(configDir)
	hasData := dirExists(dataDir)

	if hasConfig || hasData {
		fmt.Println()
		if hasConfig {
			fmt.Printf("  Config:  %s\n", configDir)
		}
		if hasData {
			fmt.Printf("  Data:    %s (projects database, settings, logs)\n", dataDir)
		}
		fmt.Print("\nAlso remove config and data? This cannot be undone. [y/N] ")
		if confirm() {
			if hasConfig {
				if err := os.RemoveAll(configDir); err != nil {
					fmt.Fprintf(os.Stderr, "Error removing config: %v\n", err)
				} else {
					fmt.Printf("Removed %s\n", configDir)
				}
			}
			if hasData {
				if err := os.RemoveAll(dataDir); err != nil {
					fmt.Fprintf(os.Stderr, "Error removing data: %v\n", err)
				} else {
					fmt.Printf("Removed %s\n", dataDir)
				}
			}
		} else {
			fmt.Println("Kept config and data. You can reinstall later and your settings will be preserved.")
		}
	}

	fmt.Println("\nWarpspawn uninstalled.")
}

func ensureDataDirs(home string) {
	dirs := []string{
		filepath.Join(home, ".config", "warpspawn"),
		filepath.Join(home, ".local", "share", "warpspawn"),
		filepath.Join(home, ".local", "share", "warpspawn", "projects"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot create %s: %v\n", dir, err)
		}
	}
}

func checkPath(binDir string) {
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == binDir {
			return // already in PATH
		}
	}

	// Detect shell
	shell := filepath.Base(os.Getenv("SHELL"))
	rcFile := "~/.bashrc"
	switch shell {
	case "zsh":
		rcFile = "~/.zshrc"
	case "fish":
		rcFile = "~/.config/fish/config.fish"
	}

	fmt.Printf("\nNote: %s is not in your PATH.\n", binDir)
	if shell == "fish" {
		fmt.Printf("Add this to %s:\n  fish_add_path %s\n", rcFile, binDir)
	} else {
		fmt.Printf("Add this to %s:\n  export PATH=\"$PATH:%s\"\n", rcFile, binDir)
	}
	fmt.Println("Then restart your terminal or run: source", rcFile)
}

func confirm() bool {
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
