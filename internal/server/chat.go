package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/haste-lab/warpspawn/internal/agent"
	"github.com/haste-lab/warpspawn/internal/config"
	"github.com/haste-lab/warpspawn/internal/core"
	"github.com/haste-lab/warpspawn/internal/guard"
	"github.com/haste-lab/warpspawn/internal/provider"
)

// ChatMessage represents a single message in the shaping conversation.
type ChatMessage struct {
	Role      string `json:"role"`    // "user", "assistant", "system"
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// ProjectChat manages the shaping conversation for a project.
type ProjectChat struct {
	ProjectID string        `json:"project_id"`
	Mode      string        `json:"mode"` // "quick" or "guided"
	Messages  []ChatMessage `json:"messages"`
	Phase     string        `json:"phase"` // "shaping", "plan-review", "approved"
}

// In-memory chat sessions (persisted to disk per project)
var (
	chatSessions = make(map[string]*ProjectChat)
	chatMu       sync.Mutex // full mutex, not RW — chat handlers mutate the session
)

// Active build tracking — prevents concurrent builds on same project
var (
	activeBuilds = make(map[string]context.CancelFunc)
	buildMu      sync.Mutex
)

type chatRequest struct {
	Message string `json:"message"`
	Mode    string `json:"mode,omitempty"` // "quick" or "guided" — only on first message
}

type chatResponse struct {
	Reply    string        `json:"reply"`
	Phase    string        `json:"phase"`
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

func (s *Server) handleProjectChat(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		http.Error(w, "project ID required", http.StatusBadRequest)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Lock per-project chat — prevents race conditions from concurrent requests
	chatMu.Lock()
	defer chatMu.Unlock()

	// Get or create chat session
	chat := getOrCreateChat(projectID, req.Mode)

	// Load project brief for context
	paths := config.DefaultPaths()
	projectRoot := filepath.Join(paths.ProjectDir, projectID)
	briefData, _ := os.ReadFile(filepath.Join(projectRoot, "docs/project-brief.md"))
	brief := string(briefData)

	// Add user message
	if req.Message != "" {
		chat.Messages = append(chat.Messages, ChatMessage{
			Role:      "user",
			Content:   req.Message,
			Timestamp: time.Now().UnixMilli(),
		})
	}

	// Pick model for MC role
	model := s.cfg.Roles["mission-control"].Model
	if model == "" {
		model = "qwen3:8b"
	}

	// Check if user is approving the plan — handle without LLM call
	if req.Message != "" && chat.Phase == "plan-review" {
		lower := strings.ToLower(req.Message)
		if lower == "go" || lower == "approve" || lower == "approved" || lower == "start" ||
			lower == "yes" || lower == "y" || lower == "ok" ||
			strings.Contains(lower, "looks good") || strings.Contains(lower, "start building") ||
			strings.Contains(lower, "approve") {

			chat.Phase = "approved"
			taskCount := s.createTasksFromPlan(projectID, projectRoot, chat)

			replyText := fmt.Sprintf("Plan approved. Created %d tasks. Click **Start Building** to begin autonomous execution.", taskCount)
			chat.Messages = append(chat.Messages, ChatMessage{
				Role:      "assistant",
				Content:   replyText,
				Timestamp: time.Now().UnixMilli(),
			})
			saveChat(projectRoot, chat)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(chatResponse{
				Reply:    replyText,
				Phase:    chat.Phase,
				Model:    model,
				Messages: chat.Messages,
			})
			return
		}
	}

	// Build LLM messages
	llmMessages := buildShapingMessages(chat, brief)

	// Pick a provider
	prov := s.pickProvider()
	if prov == nil {
		http.Error(w, "no LLM provider available", http.StatusServiceUnavailable)
		return
	}

	// Call LLM
	stream, err := prov.Complete(r.Context(), llmMessages, provider.CompletionOptions{
		Model: model,
	})
	if err != nil {
		http.Error(w, "LLM error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Collect full response
	var reply strings.Builder
	for chunk := range stream {
		if chunk.Error != nil {
			slog.Error("chat LLM error", "error", chunk.Error)
			break
		}
		reply.WriteString(chunk.Text)
	}

	replyText := reply.String()

	// Check if MC produced a plan (contains task headings)
	if strings.Contains(replyText, "TASK-") || strings.Contains(replyText, "## Plan") || strings.Contains(replyText, "## Tasks") {
		chat.Phase = "plan-review"
	}

	// Add assistant reply
	chat.Messages = append(chat.Messages, ChatMessage{
		Role:      "assistant",
		Content:   replyText,
		Timestamp: time.Now().UnixMilli(),
	})

	// Persist chat
	saveChat(projectRoot, chat)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResponse{
		Reply:    replyText,
		Phase:    chat.Phase,
		Model:    model,
		Messages: chat.Messages,
	})
}

func (s *Server) handleGetChat(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	paths := config.DefaultPaths()
	projectRoot := filepath.Join(paths.ProjectDir, projectID)

	chat := loadChat(projectRoot, projectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chat)
}

func buildShapingMessages(chat *ProjectChat, brief string) []provider.Message {
	var systemPrompt string

	if chat.Mode == "guided" {
		systemPrompt = `You are Mission Control, the orchestrator of an autonomous software delivery framework.

The user has provided a project brief. Your job during the shaping phase:
1. Ask clarifying questions about scope, tech choices, and constraints
2. When relevant, propose alternatives with trade-offs (e.g., "Option A: X. Option B: Y. I'd recommend A because...")
3. Once scope is clear, produce a numbered plan with task headings and one-line descriptions
4. Format the plan clearly with "## Plan" heading and "TASK-XXX: Title — description" format
5. Ask the user to approve, edit, or ask questions about the plan

Keep questions focused and concise. Group related questions. Don't ask more than 3-5 questions at once.`
	} else {
		systemPrompt = `You are Mission Control, the orchestrator of an autonomous software delivery framework.

The user has provided a project brief. Produce a high-level plan immediately:
1. Read the brief carefully
2. Produce a numbered plan with task headings and one-line descriptions
3. Format with "## Plan" heading and "TASK-XXX: Title — description" format
4. Keep tasks bounded and concrete (each should be implementable by one agent in one session)
5. Order tasks by dependency (scaffold first, then core features, then polish)
6. End with: "Approve this plan to start building, or tell me what to change."

Do NOT ask questions first — go straight to the plan.`
	}

	messages := []provider.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "Project brief:\n\n" + brief},
	}

	// Add conversation history (skip the first user message since brief is already included)
	for i, msg := range chat.Messages {
		if i == 0 && msg.Role == "user" {
			continue // skip initial "start" message
		}
		messages = append(messages, provider.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return messages
}

func (s *Server) pickProvider() provider.Provider {
	// Prefer the provider configured for mission-control
	mcCfg := s.cfg.Roles["mission-control"]
	if prov, ok := s.providers[mcCfg.Provider]; ok {
		return prov
	}
	// Fallback: first available
	for _, prov := range s.providers {
		return prov
	}
	return nil
}

func (s *Server) createTasksFromPlan(projectID, projectRoot string, chat *ProjectChat) int {
	// Find the last assistant message containing the plan
	var planText string
	for i := len(chat.Messages) - 1; i >= 0; i-- {
		msg := chat.Messages[i]
		if msg.Role == "assistant" && (strings.Contains(msg.Content, "TASK-") || strings.Contains(msg.Content, "## Plan")) {
			planText = msg.Content
			break
		}
	}
	if planText == "" {
		return 0
	}

	// Parse task lines: "TASK-XXX: Title — description" or "N. Title — description"
	taskNum := 0
	var backlogLines []string

	for _, line := range strings.Split(planText, "\n") {
		line = strings.TrimSpace(line)
		// Match patterns like "TASK-001:", "1.", "- TASK-"
		if !isTaskLine(line) {
			continue
		}

		taskNum++
		taskID := fmt.Sprintf("TASK-%03d", taskNum)
		title, description := parseTaskLine(line)
		if title == "" {
			continue
		}

		// Write task file
		taskContent := fmt.Sprintf(`# Task

## Metadata
- Task ID: %s
- Title: %s
- Status: ready-for-build
- Priority: P%d
- Owner Role: Builder
- Source Files: app/
- Last Updated: %s

## Objective
%s

## Acceptance Criteria
- [ ] Implementation matches the described scope
- [ ] Code runs without errors

## Implementation Notes
pending

## Validation
pending

## Handoff
- Next Role: Reviewer/QA
`, taskID, title, min(taskNum-1, 3), time.Now().Format("2006-01-02"),
			coalesce(description, title))

		writeProjectFile(projectRoot, fmt.Sprintf("tasks/%s.md", strings.ToLower(taskID)), taskContent)

		backlogLines = append(backlogLines, fmt.Sprintf("| %s | %s | ready-for-build | P%d | Builder | - | brief | - |",
			taskID, title, min(taskNum-1, 3)))
	}

	// Update backlog
	if len(backlogLines) > 0 {
		backlogContent := "## Backlog\n\n| ID | Title | Status | Priority | Owner Role | Depends On | Source | Notes |\n|---|---|---|---|---|---|---|---|\n"
		backlogContent += strings.Join(backlogLines, "\n") + "\n"
		writeProjectFile(projectRoot, "backlog/backlog.md", backlogContent)
	}

	// Update project state
	stateContent := fmt.Sprintf(`# Project State Snapshot

- Project: %s
- Project Lifecycle: active
- Current Stage: ready for build
- Autonomous Pickup: enabled
- Current Epoch: %s
`, projectID, projectID+"-"+time.Now().Format("2006-01-02"))
	writeProjectFile(projectRoot, "status/project-state.md", stateContent)

	// Update SQLite
	if s.db != nil {
		s.db.UpsertProject(projectID, projectRoot, projectID, "active", "ready for build", "")
		for i := 1; i <= taskNum; i++ {
			taskID := fmt.Sprintf("TASK-%03d", i)
			s.db.UpsertTask(projectID, taskID, filepath.Join(projectRoot, "tasks", strings.ToLower(taskID)+".md"),
				"", "ready-for-build", fmt.Sprintf("P%d", min(i-1, 3)), "Builder", "standard")
		}
	}

	// Git commit
	initGit(projectRoot) // re-commit with new tasks

	slog.Info("created tasks from plan", "project", projectID, "tasks", taskNum)
	return taskNum
}

func isTaskLine(line string) bool {
	if strings.HasPrefix(line, "TASK-") {
		return true
	}
	// Numbered list: "1.", "2.", etc.
	for i, ch := range line {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '.' && i > 0 {
			return true
		}
		break
	}
	// Bullet with TASK reference
	if strings.HasPrefix(line, "- ") && strings.Contains(line, "TASK-") {
		return true
	}
	if strings.HasPrefix(line, "- **") {
		return true
	}
	return false
}

func parseTaskLine(line string) (title, description string) {
	// Strip leading patterns: "TASK-001: ", "1. ", "- ", "- **"
	clean := line
	clean = strings.TrimPrefix(clean, "- ")
	clean = strings.TrimPrefix(clean, "**")

	// Remove "TASK-XXX: " prefix
	if idx := strings.Index(clean, ": "); idx >= 0 && idx < 20 {
		clean = clean[idx+2:]
	}

	// Remove leading "N. "
	for i, ch := range clean {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '.' && i > 0 {
			clean = strings.TrimSpace(clean[i+1:])
			break
		}
		break
	}

	clean = strings.TrimSuffix(clean, "**")
	clean = strings.TrimSpace(clean)

	// Split on " — " or " - " for title/description
	for _, sep := range []string{" — ", " - ", ": "} {
		if idx := strings.Index(clean, sep); idx > 0 {
			return strings.TrimSpace(clean[:idx]), strings.TrimSpace(clean[idx+len(sep):])
		}
	}

	return clean, ""
}

func getOrCreateChat(projectID, mode string) *ProjectChat {
	chatMu.Lock()
	defer chatMu.Unlock()

	if chat, ok := chatSessions[projectID]; ok {
		return chat
	}

	// Try loading from disk
	paths := config.DefaultPaths()
	projectRoot := filepath.Join(paths.ProjectDir, projectID)
	chat := loadChat(projectRoot, projectID)
	if chat.Mode == "" {
		if mode == "" {
			mode = "quick"
		}
		chat.Mode = mode
		chat.Phase = "shaping"
	}
	chatSessions[projectID] = chat
	return chat
}

func loadChat(projectRoot, projectID string) *ProjectChat {
	chatPath := filepath.Join(projectRoot, "status", "shaping-chat.json")
	data, err := os.ReadFile(chatPath)
	if err != nil {
		return &ProjectChat{ProjectID: projectID, Messages: []ChatMessage{}}
	}
	var chat ProjectChat
	if err := json.Unmarshal(data, &chat); err != nil {
		return &ProjectChat{ProjectID: projectID, Messages: []ChatMessage{}}
	}
	return &chat
}

func saveChat(projectRoot string, chat *ProjectChat) {
	chatPath := filepath.Join(projectRoot, "status", "shaping-chat.json")
	data, _ := json.MarshalIndent(chat, "", "  ")
	tmpPath := chatPath + fmt.Sprintf(".tmp.%d", os.Getpid())
	os.WriteFile(tmpPath, data, 0644)
	os.Rename(tmpPath, chatPath)

	chatMu.Lock()
	chatSessions[chat.ProjectID] = chat
	chatMu.Unlock()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Server) handleStartBuild(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		http.Error(w, "project ID required", http.StatusBadRequest)
		return
	}

	// Check for concurrent build on this project
	buildMu.Lock()
	if _, running := activeBuilds[projectID]; running {
		buildMu.Unlock()
		http.Error(w, "build already running for this project", http.StatusConflict)
		return
	}

	paths := config.DefaultPaths()
	projectRoot := filepath.Join(paths.ProjectDir, projectID)

	// Pick provider and model
	prov := s.pickProvider()
	if prov == nil {
		buildMu.Unlock()
		http.Error(w, "no LLM provider available", http.StatusServiceUnavailable)
		return
	}

	// Run orchestrator with tracking
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	activeBuilds[projectID] = cancel
	buildMu.Unlock()

	go func() {
		defer cancel()
		defer func() {
			buildMu.Lock()
			delete(activeBuilds, projectID)
			buildMu.Unlock()
		}()

		workflow := core.DefaultWorkflow
		budget := guard.NewBudget(paths.DataDir)

		orch := &core.Orchestrator{
			Workflow:      &workflow,
			Provider:      prov,
			DB:            s.db,
			Budget:        budget,
			BuilderModel:  s.cfg.Roles["builder"].Model,
			ReviewerModel: s.cfg.Roles["reviewer-qa"].Model,
			MaxTools:      s.cfg.Execution.MaxToolCalls,
			TimeoutS:      s.cfg.Execution.AgentTimeoutS,
			ShellMode:     s.cfg.Execution.ShellMode,
			OnEvent: func(event agent.StreamEvent) {
				s.Broadcast(SSEEvent{
					Type: "agent." + event.Type,
					Data: map[string]interface{}{
						"project_id": projectID,
						"content":    event.Text,
						"summary":    event.Summary,
					},
				})
			},
		}

		result := orch.RunProject(ctx, projectRoot)
		s.Broadcast(SSEEvent{
			Type: "run.complete",
			Data: map[string]interface{}{
				"project_id": projectID,
				"action":     result.Action.Kind,
				"state":      result.StateUpdate,
			},
		})
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started", "project_id": projectID})
}
