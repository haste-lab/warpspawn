package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/haste-lab/warpspawn/internal/provider"
)

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

// ExecuteTool runs a tool call and returns the result.
// projectRoot is the absolute path to the project directory — all file paths are resolved relative to it.
func ExecuteTool(projectRoot string, call provider.ToolCall, commandTimeout time.Duration) ToolResult {
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
		return executeRunCommand(projectRoot, call.ID, args, commandTimeout)
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

func executeRunCommand(projectRoot, callID string, args map[string]interface{}, timeout time.Duration) ToolResult {
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

	cmd := exec.Command(command, cmdArgs...)
	cmd.Dir = projectRoot

	// Combine stdout and stderr
	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil {
		result = fmt.Sprintf("%s\nCommand error: %v", result, err)
	}

	// Truncate very long output
	if len(result) > 32000 {
		result = result[:32000] + "\n... (output truncated)"
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
