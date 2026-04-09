package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/haste-lab/warpspawn/internal/provider"
)

// ShellMode controls what commands agents can execute.
type ShellMode string

const (
	ShellUnrestricted ShellMode = "unrestricted"
	ShellRestricted   ShellMode = "restricted"
	ShellApproval     ShellMode = "approval" // not implemented in v1 — blocks all commands
)

// AllowedCommands is the default allowlist for restricted shell mode.
var AllowedCommands = map[string]bool{
	"node": true, "npm": true, "npx": true,
	"python": true, "python3": true, "pip": true, "pip3": true,
	"go": true, "cargo": true, "rustc": true, "make": true,
	"git": true,
	"ls": true, "cat": true, "head": true, "tail": true,
	"mkdir": true, "cp": true, "mv": true, "touch": true, "echo": true,
	"test": true, "wc": true, "sort": true, "uniq": true,
	"grep": true, "find": true, "dirname": true, "basename": true,
	"chmod": true, "rm": true, // rm allowed but rm -rf / blocked below
	"sh": true, "bash": true, // needed for scripts but args are checked
}

// BlockedPatterns are dangerous command patterns blocked even in unrestricted mode.
var BlockedPatterns = []string{
	"rm -rf /",
	"rm -rf /*",
	"rm -rf ~",
	"sudo ",
	"su ",
	"chmod 777 /",
	"mkfs",
	"dd if=",
	"> /dev/sd",
	":(){ :|:& };:", // fork bomb
}

// ValidateCommand checks if a command is allowed under the given shell mode.
func ValidateCommand(command string, args []string, mode ShellMode) error {
	fullCmd := command + " " + strings.Join(args, " ")

	// Always block dangerous patterns regardless of mode
	for _, pattern := range BlockedPatterns {
		if strings.Contains(fullCmd, pattern) {
			return fmt.Errorf("blocked: dangerous command pattern %q", pattern)
		}
	}

	if mode == ShellApproval {
		return fmt.Errorf("blocked: shell mode is 'approval' (not implemented — all commands blocked)")
	}

	if mode == ShellRestricted {
		baseName := filepath.Base(command)
		if !AllowedCommands[baseName] {
			return fmt.Errorf("blocked: command %q not in allowlist (shell mode: restricted)", baseName)
		}

		// Block network commands
		for _, blocked := range []string{"curl", "wget", "ssh", "scp", "nc", "ncat", "netcat"} {
			if baseName == blocked {
				return fmt.Errorf("blocked: network command %q not allowed in restricted mode", baseName)
			}
		}

		// Block shell -c entirely — too many bypass vectors (subshells, backticks, process substitution)
		if (baseName == "bash" || baseName == "sh") && containsFlag(args, "-c") {
			return fmt.Errorf("blocked: %s -c is not allowed in restricted mode (use individual commands instead)", baseName)
		}

		// Block interpreter eval flags — prevents network calls via python/node
		if (baseName == "python" || baseName == "python3" || baseName == "node") && containsFlag(args, "-c", "-e", "--eval") {
			return fmt.Errorf("blocked: %s with -c/-e flag not allowed in restricted mode (can execute arbitrary code)", baseName)
		}

		// Validate arguments for filesystem commands — block access outside project directory
		if isFilesystemCommand(baseName) {
			for _, arg := range args {
				if err := validateCommandArg(arg); err != nil {
					return fmt.Errorf("blocked: %s argument %v", baseName, err)
				}
			}
		}
	}

	return nil
}

// containsFlag checks if any of the given flags appear in the argument list.
func containsFlag(args []string, flags ...string) bool {
	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag {
				return true
			}
		}
	}
	return false
}

// isFilesystemCommand returns true if the command can read/write arbitrary files.
func isFilesystemCommand(baseName string) bool {
	switch baseName {
	case "cat", "head", "tail", "cp", "mv", "rm", "chmod", "ls", "find", "grep":
		return true
	}
	return false
}

// validateCommandArg checks that a command argument doesn't reference paths outside the project.
func validateCommandArg(arg string) error {
	// Skip flags
	if strings.HasPrefix(arg, "-") {
		return nil
	}
	// Block absolute paths (must be relative to project root)
	if strings.HasPrefix(arg, "/") {
		return fmt.Errorf("absolute path %q not allowed (must be relative to project directory)", arg)
	}
	// Block parent traversal
	if strings.Contains(arg, "..") {
		return fmt.Errorf("path traversal %q not allowed", arg)
	}
	return nil
}

// extractCommandNames parses a shell command string and returns the base command names.
// Handles: "cmd1 && cmd2", "cmd1 || cmd2", "cmd1; cmd2", "cmd1 | cmd2"
func extractCommandNames(payload string) []string {
	var names []string
	// Split on shell operators
	for _, sep := range []string{"&&", "||", ";", "|"} {
		payload = strings.ReplaceAll(payload, sep, "\n")
	}
	for _, part := range strings.Split(payload, "\n") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// First token is the command
		fields := strings.Fields(part)
		if len(fields) > 0 {
			name := filepath.Base(fields[0])
			names = append(names, name)
		}
	}
	return names
}

// ToolResult is the outcome of executing a tool call.
type ToolResult struct {
	ToolCallID string
	Content    string
	Error      error
}

// BuiltinTools returns the tool definitions available to agents.
func BuiltinTools() []provider.ToolDef {
	return []provider.ToolDef{
		{
			Name:        "read_file",
			Description: "Read the contents of a file. Returns the file content as text.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path to the file within the project directory",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file. Creates the file if it doesn't exist, overwrites if it does.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path to the file within the project directory",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The full content to write to the file",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "list_files",
			Description: "List files and directories at a given path. Returns one entry per line.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative directory path within the project directory. Use '.' for project root.",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "run_command",
			Description: "Run a shell command in the project directory. Returns stdout and stderr.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The command to execute",
					},
					"args": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Command arguments",
					},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "task_complete",
			Description: "Signal that you have finished the task. Call this when all work is done.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"summary": map[string]interface{}{
						"type":        "string",
						"description": "Brief summary of what was accomplished",
					},
				},
				"required": []string{"summary"},
			},
		},
	}
}

// ExecuteConfig holds settings for tool execution.
type ExecuteConfig struct {
	ProjectRoot    string
	CommandTimeout time.Duration
	ShellMode      ShellMode
}

// ExecuteTool runs a tool call and returns the result.
func ExecuteTool(cfg ExecuteConfig, call provider.ToolCall) ToolResult {
	projectRoot := cfg.ProjectRoot
	commandTimeout := cfg.CommandTimeout
	if commandTimeout <= 0 {
		commandTimeout = 30 * time.Second
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return ToolResult{
			ToolCallID: call.ID,
			Content:    fmt.Sprintf("Error parsing tool arguments: %v", err),
			Error:      err,
		}
	}

	switch call.Name {
	case "read_file":
		return executeReadFile(projectRoot, call.ID, args)
	case "write_file":
		return executeWriteFile(projectRoot, call.ID, args)
	case "list_files":
		return executeListFiles(projectRoot, call.ID, args)
	case "run_command":
		return executeRunCommand(projectRoot, call.ID, args, commandTimeout, cfg.ShellMode)
	case "task_complete":
		summary, _ := args["summary"].(string)
		return ToolResult{
			ToolCallID: call.ID,
			Content:    fmt.Sprintf("Task complete: %s", summary),
		}
	default:
		return ToolResult{
			ToolCallID: call.ID,
			Content:    fmt.Sprintf("Unknown tool: %s", call.Name),
			Error:      fmt.Errorf("unknown tool: %s", call.Name),
		}
	}
}

func executeReadFile(projectRoot, callID string, args map[string]interface{}) ToolResult {
	relPath, _ := args["path"].(string)
	absPath, err := safePath(projectRoot, relPath)
	if err != nil {
		return ToolResult{ToolCallID: callID, Content: err.Error(), Error: err}
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return ToolResult{ToolCallID: callID, Content: fmt.Sprintf("Error reading file: %v", err), Error: err}
	}

	return ToolResult{ToolCallID: callID, Content: string(data)}
}

func executeWriteFile(projectRoot, callID string, args map[string]interface{}) ToolResult {
	relPath, _ := args["path"].(string)
	content, _ := args["content"].(string)

	absPath, err := safePath(projectRoot, relPath)
	if err != nil {
		return ToolResult{ToolCallID: callID, Content: err.Error(), Error: err}
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return ToolResult{ToolCallID: callID, Content: fmt.Sprintf("Error creating directory: %v", err), Error: err}
	}

	// Atomic write: write to temp file, then rename
	tmpPath := absPath + fmt.Sprintf(".tmp.%d", os.Getpid())
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return ToolResult{ToolCallID: callID, Content: fmt.Sprintf("Error writing file: %v", err), Error: err}
	}
	if err := os.Rename(tmpPath, absPath); err != nil {
		os.Remove(tmpPath)
		return ToolResult{ToolCallID: callID, Content: fmt.Sprintf("Error renaming file: %v", err), Error: err}
	}

	return ToolResult{ToolCallID: callID, Content: fmt.Sprintf("Written %d bytes to %s", len(content), relPath)}
}

func executeListFiles(projectRoot, callID string, args map[string]interface{}) ToolResult {
	relPath, _ := args["path"].(string)
	absPath, err := safePath(projectRoot, relPath)
	if err != nil {
		return ToolResult{ToolCallID: callID, Content: err.Error(), Error: err}
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return ToolResult{ToolCallID: callID, Content: fmt.Sprintf("Error listing directory: %v", err), Error: err}
	}

	var lines []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}

	return ToolResult{ToolCallID: callID, Content: strings.Join(lines, "\n")}
}

func executeRunCommand(projectRoot, callID string, args map[string]interface{}, timeout time.Duration, shellMode ShellMode) ToolResult {
	command, _ := args["command"].(string)
	if command == "" {
		return ToolResult{ToolCallID: callID, Content: "Error: empty command", Error: fmt.Errorf("empty command")}
	}

	var cmdArgs []string
	if rawArgs, ok := args["args"].([]interface{}); ok {
		for _, a := range rawArgs {
			if s, ok := a.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}

	// Validate command against shell mode
	if err := ValidateCommand(command, cmdArgs, shellMode); err != nil {
		return ToolResult{ToolCallID: callID, Content: fmt.Sprintf("Command blocked: %v", err), Error: err}
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, cmdArgs...)
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	result := string(output)

	if ctx.Err() == context.DeadlineExceeded {
		result = fmt.Sprintf("%s\nCommand timed out after %s", result, timeout)
		return ToolResult{ToolCallID: callID, Content: result, Error: fmt.Errorf("command timed out")}
	}

	if err != nil {
		result = fmt.Sprintf("%s\nCommand error: %v", result, err)
	}

	// Truncate very long output to save context window
	if len(result) > 16000 {
		result = result[:16000] + "\n... (output truncated to save context)"
	}

	return ToolResult{ToolCallID: callID, Content: result}
}

// safePath resolves a relative path against the project root and ensures it doesn't escape.
func safePath(projectRoot, relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("empty file path")
	}

	absPath := filepath.Join(projectRoot, relPath)
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Ensure the resolved path is within the project root
	absRoot, _ := filepath.Abs(projectRoot)
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		return "", fmt.Errorf("path %q escapes project directory", relPath)
	}

	return absPath, nil
}
