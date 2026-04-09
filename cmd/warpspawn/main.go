package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/haste-lab/warpspawn/internal/provider"
)

var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "Enable debug logging")
	showVersion := flag.Bool("version", false, "Print version and exit")
	port := flag.Int("port", 9320, "HTTP server port")
	noBrowser := flag.Bool("no-browser", false, "Don't open browser on startup")
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

	// Verify Ollama is reachable (quick check)
	ollama := provider.NewOllamaProvider("")
	if err := ollama.HealthCheck(context.Background()); err != nil {
		slog.Warn("Ollama not reachable", "error", err)
	} else {
		models, err := ollama.ListModels(context.Background())
		if err == nil {
			slog.Info("Ollama connected", "models", len(models))
		}
	}

	_ = port
	_ = noBrowser

	fmt.Printf(`
 __      __
 \ \    / /_ _ _ _ _ __ ___ _ __  __ ___ __ ___ _
  \ \/\/ / _` + "`" + `| '_| '_ (_-< '_ \/ _` + "`" + `  \ V  V / ' \
   \_/\_/\__,_|_| | .__/__/ .__/\__,_|\_/\_/|_||_|
                  |_|     |_|   %s

  Server:  http://localhost:%d
  Press Ctrl+C to stop.

`, version, *port)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	sig := <-sigCh
	slog.Info("Shutting down", "signal", sig)
}
