package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/haste-lab/warpspawn/internal/config"
	"github.com/haste-lab/warpspawn/internal/db"
	"github.com/haste-lab/warpspawn/internal/provider"
	"github.com/haste-lab/warpspawn/internal/server"
)

var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "Enable debug logging")
	showVersion := flag.Bool("version", false, "Print version and exit")
	port := flag.Int("port", 9320, "HTTP server port")
	noBrowser := flag.Bool("no-browser", false, "Don't open browser on startup")
	host := flag.String("host", "127.0.0.1", "Bind address (use 0.0.0.0 for LAN access)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("warpspawn %s\n", version)
		os.Exit(0)
	}

	// Configure logging
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	// Paths
	paths := config.DefaultPaths()
	os.MkdirAll(paths.ConfigDir, 0755)
	os.MkdirAll(paths.DataDir, 0755)
	os.MkdirAll(paths.ProjectDir, 0755)

	// Check PID file for multi-instance prevention
	pidPath := filepath.Join(paths.DataDir, "warpspawn.pid")
	if err := checkPidFile(pidPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	writePidFile(pidPath)
	defer os.Remove(pidPath)

	// Load config
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Open database
	database, err := db.Open(paths.DataDir)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	database.Backup()

	// Check Ollama
	ollama := provider.NewOllamaProvider(cfg.Providers["ollama"].BaseURL)
	if err := ollama.HealthCheck(context.Background()); err != nil {
		slog.Warn("Ollama not reachable", "error", err)
	} else {
		models, _ := ollama.ListModels(context.Background())
		slog.Info("Ollama connected", "models", len(models))
	}

	// Generate session token
	token := config.GenerateSessionToken()

	// Warn if binding to all interfaces
	if *host == "0.0.0.0" {
		slog.Warn("Server accessible on all network interfaces. API token required for all requests.")
	}

	_ = host // TODO: pass to server for non-localhost binding

	// Start HTTP server
	srv := server.New(*port, token, database, cfg)
	actualPort, shutdown, err := srv.Start(context.Background())
	if err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}
	defer shutdown()

	url := fmt.Sprintf("http://localhost:%d?token=%s", actualPort, token)

	// Print banner
	fmt.Printf(`
 __      __
 \ \    / /_ _ _ _ _ __ ___ _ __  __ ___ __ ___ _
  \ \/\/ / _`+"`"+` | '_| '_ (_-< '_ \/ _`+"`"+`  \ V  V / ' \
   \_/\_/\__,_|_| | .__/__/ .__/\__,_|\_/\_/|_||_|
                  |_|     |_|   %s

  Server:  %s
`, version, url)

	// Open browser
	if !*noBrowser {
		if err := openBrowser(url); err != nil {
			fmt.Printf("  Browser: could not open automatically.\n")
			fmt.Printf("           Open this URL manually: %s\n", url)
		} else {
			fmt.Printf("  Browser: opening...\n")
		}
	}

	fmt.Printf("\n  Press Ctrl+C to stop.\n\n")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	sig := <-sigCh
	slog.Info("Shutting down", "signal", sig)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Start()
}

func checkPidFile(pidPath string) error {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return nil // no PID file — OK
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return nil // corrupt PID file — overwrite
	}

	// Check if the process is still running
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}

	// On Unix, FindProcess always succeeds. Send signal 0 to check.
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return nil // process not running — stale PID file
	}

	return fmt.Errorf("Warpspawn already running (PID %d). If this is wrong, delete %s", pid, pidPath)
}

func writePidFile(pidPath string) {
	os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}
