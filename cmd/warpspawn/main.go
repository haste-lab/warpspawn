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
	"strings"
	"syscall"

	"github.com/haste-lab/warpspawn/internal/agent"
	"github.com/haste-lab/warpspawn/internal/config"
	"github.com/haste-lab/warpspawn/internal/core"
	"github.com/haste-lab/warpspawn/internal/db"
	"github.com/haste-lab/warpspawn/internal/guard"
	"github.com/haste-lab/warpspawn/internal/provider"
	"github.com/haste-lab/warpspawn/internal/server"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "run":
			runCmd(os.Args[2:])
			return
		case "install":
			installCmd()
			return
		case "uninstall":
			uninstallCmd()
			return
		case "help", "--help", "-h":
			printHelp()
			return
		}
	}
	serveCmd(os.Args[1:])
}

func printHelp() {
	fmt.Printf(`Warpspawn %s — Autonomous agentic software delivery

Usage:
  warpspawn                    Start the UI server (default)
  warpspawn run <project>      Run one orchestration cycle on a project
  warpspawn install            Install to ~/.local/bin for global access
  warpspawn uninstall          Remove installed binary and optionally data

Server options:
  --port=N          HTTP port (default: 9320)
  --no-browser      Don't open browser on startup
  --host=ADDR       Bind address (default: 127.0.0.1)
  --debug           Enable debug logging

Run options:
  --provider=NAME   LLM provider: ollama, openai, anthropic (default: ollama)
  --model=NAME      Model to use (default: from config)
  --debug           Enable debug logging

`, version)
}

// runCmd handles: warpspawn run <project-path> [--provider ollama] [--model qwen2.5-coder:7b]
func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	debug := fs.Bool("debug", false, "Enable debug logging")
	providerName := fs.String("provider", "ollama", "LLM provider (ollama, openai, anthropic)")
	model := fs.String("model", "", "Model to use (default: auto from config)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: warpspawn run <project-path> [--provider ollama] [--model qwen3:8b]\n")
		os.Exit(1)
	}
	projectRoot, _ := filepath.Abs(fs.Arg(0))

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	paths := config.DefaultPaths()
	os.MkdirAll(paths.DataDir, 0755)

	cfg, _ := config.Load(paths.ConfigDir)

	database, err := db.Open(paths.DataDir)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Resolve provider
	var llmProvider provider.Provider
	switch *providerName {
	case "ollama":
		baseURL := cfg.Providers["ollama"].BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		llmProvider = provider.NewOllamaProvider(baseURL)
	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			fmt.Fprintf(os.Stderr, "Error: OPENAI_API_KEY not set\n")
			os.Exit(1)
		}
		llmProvider = provider.NewOpenAIProvider(apiKey)
	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			fmt.Fprintf(os.Stderr, "Error: ANTHROPIC_API_KEY not set\n")
			os.Exit(1)
		}
		llmProvider = provider.NewAnthropicProvider(apiKey)
	default:
		fmt.Fprintf(os.Stderr, "Unknown provider: %s\n", *providerName)
		os.Exit(1)
	}

	// Health check
	if err := llmProvider.HealthCheck(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Provider %s not reachable: %v\n", *providerName, err)
		os.Exit(1)
	}

	// Resolve model
	if *model == "" {
		// Use config defaults
		roleCfg := cfg.Roles["builder"]
		if roleCfg.Model != "" {
			*model = roleCfg.Model
		} else {
			*model = "qwen2.5-coder:7b"
		}
	}

	budget := guard.NewBudget(paths.DataDir)
	workflow := core.DefaultWorkflow

	orch := &core.Orchestrator{
		Workflow:      &workflow,
		Provider:      llmProvider,
		DB:            database,
		Budget:        budget,
		MaxTools:      cfg.Execution.MaxToolCalls,
		TimeoutS:      cfg.Execution.AgentTimeoutS,
		BuilderModel:  *model,
		ReviewerModel: *model,
		OnEvent: func(event agent.StreamEvent) {
			switch event.Type {
			case "text":
				fmt.Print(event.Text)
			case "tool_call":
				if event.ToolCall != nil {
					fmt.Fprintf(os.Stderr, "\n[tool] %s\n", event.ToolCall.Name)
				}
			case "tool_result":
				if event.ToolResult != nil {
					output := event.ToolResult.Content
					if len(output) > 200 {
						output = output[:200] + "..."
					}
					fmt.Fprintf(os.Stderr, "[result] %s\n", strings.ReplaceAll(output, "\n", " "))
				}
			case "complete":
				fmt.Fprintf(os.Stderr, "\n[complete] %s\n", event.Summary)
			case "error":
				fmt.Fprintf(os.Stderr, "\n[error] %v\n", event.Error)
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintf(os.Stderr, "\nAborting...\n")
		cancel()
	}()

	result := orch.RunProject(ctx, projectRoot)

	fmt.Fprintf(os.Stderr, "\n--- Result ---\n")
	fmt.Fprintf(os.Stderr, "Project:  %s\n", result.ProjectID)
	fmt.Fprintf(os.Stderr, "Action:   %s\n", result.Action.Kind)
	fmt.Fprintf(os.Stderr, "State:    %s\n", result.StateUpdate)
	if result.AgentResult != nil {
		fmt.Fprintf(os.Stderr, "Tools:    %d calls\n", result.AgentResult.ToolCalls)
		fmt.Fprintf(os.Stderr, "Tokens:   %d in / %d out\n", result.AgentResult.TotalUsage.InputTokens, result.AgentResult.TotalUsage.OutputTokens)
		fmt.Fprintf(os.Stderr, "Success:  %v\n", result.AgentResult.Success)
	}
	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "Error:    %v\n", result.Error)
		os.Exit(1)
	}
}

// serveCmd handles the default: start the HTTP server
func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	debug := fs.Bool("debug", false, "Enable debug logging")
	showVersion := fs.Bool("version", false, "Print version and exit")
	port := fs.Int("port", 9320, "HTTP server port")
	noBrowser := fs.Bool("no-browser", false, "Don't open browser on startup")
	host := fs.String("host", "127.0.0.1", "Bind address (use 0.0.0.0 for LAN access)")
	fs.Parse(args)

	if *showVersion {
		fmt.Printf("warpspawn %s\n", version)
		os.Exit(0)
	}

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	paths := config.DefaultPaths()
	os.MkdirAll(paths.ConfigDir, 0755)
	os.MkdirAll(paths.DataDir, 0755)
	os.MkdirAll(paths.ProjectDir, 0755)

	pidPath := filepath.Join(paths.DataDir, "warpspawn.pid")
	if err := checkPidFile(pidPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	writePidFile(pidPath)
	defer os.Remove(pidPath)

	cfg, err := config.Load(paths.ConfigDir)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	database, err := db.Open(paths.DataDir)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	database.Backup()

	// Build provider map for the server
	providers := make(map[string]provider.Provider)
	ollamaURL := cfg.Providers["ollama"].BaseURL
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	ollama := provider.NewOllamaProvider(ollamaURL)
	if err := ollama.HealthCheck(context.Background()); err != nil {
		slog.Warn("Ollama not reachable", "error", err)
	} else {
		models, _ := ollama.ListModels(context.Background())
		slog.Info("Ollama connected", "models", len(models))
		providers["ollama"] = ollama
	}
	// Cloud providers are added if API keys are available (from env for now)
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		providers["openai"] = provider.NewOpenAIProvider(key)
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		providers["anthropic"] = provider.NewAnthropicProvider(key)
	}

	token := config.GenerateSessionToken()

	if *host == "0.0.0.0" {
		slog.Warn("Server accessible on all network interfaces. API token required for all requests.")
	}
	_ = host

	srv := server.New(*port, token, database, cfg, paths.ConfigDir, providers)
	actualPort, shutdown, err := srv.Start(context.Background())
	if err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}
	defer shutdown()

	url := fmt.Sprintf("http://localhost:%d?token=%s", actualPort, token)

	fmt.Printf(`
 __      __
 \ \    / /_ _ _ _ _ __ ___ _ __  __ ___ __ ___ _
  \ \/\/ / _`+"`"+` | '_| '_ (_-< '_ \/ _`+"`"+`  \ V  V / ' \
   \_/\_/\__,_|_| | .__/__/ .__/\__,_|\_/\_/|_||_|
                  |_|     |_|   %s

  Server:  %s
`, version, url)

	if !*noBrowser {
		if err := openBrowser(url); err != nil {
			fmt.Printf("  Browser: could not open automatically.\n")
			fmt.Printf("           Open this URL manually: %s\n", url)
		} else {
			fmt.Printf("  Browser: opening...\n")
		}
	}

	fmt.Printf("\n  Press Ctrl+C to stop.\n\n")

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
		return nil
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return nil
	}
	return fmt.Errorf("Warpspawn already running (PID %d). If this is wrong, delete %s", pid, pidPath)
}

func writePidFile(pidPath string) {
	os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}
