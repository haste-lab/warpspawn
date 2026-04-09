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

	// Load chat session (locked)
	chatMu.Lock()
	chat := getOrCreateChat(projectID, req.Mode)

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

	model := s.cfg.Roles["mission-control"].Model
	if model == "" {
		model = "qwen3:8b"
	}

	// Check if user wants to continue/resume building
	if req.Message != "" && chat.Phase == "approved" {
		lower := strings.ToLower(req.Message)
		if strings.Contains(lower, "proceed") || strings.Contains(lower, "continue") ||
			strings.Contains(lower, "build") || strings.Contains(lower, "resume") ||
			strings.Contains(lower, "start") || lower == "go" {

			// Check if there are unfinished tasks
			tasks := core.ListTasks(projectRoot)
			unfinished := 0
			for _, t := range tasks {
				if t.Status != "done" && t.Status != "archived" {
					unfinished++
				}
			}

			if unfinished > 0 {
				replyText := fmt.Sprintf("Resuming build — %d tasks remaining. I'll report progress as they complete.", unfinished)
				chat.Messages = append(chat.Messages, ChatMessage{
					Role:      "assistant",
					Content:   replyText,
					Timestamp: time.Now().UnixMilli(),
				})
				saveChat(projectRoot, chat)
				chatMu.Unlock()

				// Trigger the build
				go s.triggerBuild(projectID, projectRoot)

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
	}

	// Check if user is approving — handle immediately without LLM call
	if req.Message != "" && chat.Phase == "plan-review" {
		lower := strings.ToLower(req.Message)
		if lower == "go" || lower == "approve" || lower == "approved" || lower == "approved." ||
			lower == "start" || lower == "yes" || lower == "y" || lower == "ok" ||
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
			chatMu.Unlock()

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

	// Snapshot messages for LLM call, then release the lock
	llmMessages := buildShapingMessages(chat, brief)
	chatMu.Unlock()

	// Pick a provider
	prov := s.pickProvider()
	if prov == nil {
		http.Error(w, "no LLM provider available", http.StatusServiceUnavailable)
		return
	}

	// Call LLM
	stream, err := prov.Complete(r.Context(), llmMessages, provider.CompletionOptions{
		Model:       model,
		ContextSize: s.cfg.Execution.LLMContextSize,
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

	// Re-lock to update session
	chatMu.Lock()

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

	// Persist chat and release lock
	saveChat(projectRoot, chat)
	resp := chatResponse{
		Reply:    replyText,
		Phase:    chat.Phase,
		Model:    model,
		Messages: chat.Messages,
	}
	chatMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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

IMPORTANT: Always briefly acknowledge the user's input before responding. For example: "Got it — two players, same browser. Here's what I'd suggest:" or "Good point about mobile support. Let me adjust the plan:"

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

IMPORTANT: If the user provides additional context or answers questions, briefly acknowledge their input first. For example: "Got it — two players on the same browser. Here's the plan:" Then proceed with the plan.

Do NOT ask questions first — go straight to the plan.`
	}

	// Build project context: task statuses and file listing
	projectContext := buildProjectContext(chat.ProjectID)

	messages := []provider.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "Project brief:\n\n" + brief + "\n\n" + projectContext},
	}

	// Filter and prune conversation history for LLM context:
	// - Skip the initial "start" message
	// - Skip build milestone messages (🚀, ✅, 🎉, 🏁, ❌, ⚠️) — these are for the user, not the LLM
	// - Keep only the last 10 meaningful messages to stay within context limits
	var relevantMessages []provider.Message
	for i, msg := range chat.Messages {
		if i == 0 && msg.Role == "user" {
			continue
		}
		// Skip automated build status messages
		if msg.Role == "assistant" && isBuildStatusMessage(msg.Content) {
			continue
		}
		relevantMessages = append(relevantMessages, provider.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Keep last 10 messages to avoid context overflow
	if len(relevantMessages) > 10 {
		relevantMessages = relevantMessages[len(relevantMessages)-10:]
	}
	messages = append(messages, relevantMessages...)

	return messages
}

func isBuildStatusMessage(content string) bool {
	prefixes := []string{"🚀", "✅", "🎉", "🏁", "❌", "⚠️", "🔄"}
	for _, p := range prefixes {
		if strings.HasPrefix(content, p) {
			return true
		}
	}
	return false
}

func buildProjectContext(projectID string) string {
	paths := config.DefaultPaths()
	projectRoot := filepath.Join(paths.ProjectDir, projectID)

	var ctx strings.Builder

	// Task statuses
	tasks := core.ListTasks(projectRoot)
	if len(tasks) > 0 {
		ctx.WriteString("\n## Current task statuses:\n")
		done, total := 0, len(tasks)
		for _, t := range tasks {
			status := t.Status
			if status == "done" || status == "archived" {
				done++
			}
			ctx.WriteString(fmt.Sprintf("- %s: %s (%s)\n", t.TaskID, t.Title, status))
		}
		ctx.WriteString(fmt.Sprintf("\nProgress: %d/%d tasks done.\n", done, total))
	}

	// App files
	appFiles := listAppFiles(projectRoot)
	if len(appFiles) > 0 {
		ctx.WriteString("\n## Application files created:\n")
		for _, f := range appFiles {
			ctx.WriteString(fmt.Sprintf("- %s\n", f))
		}
		// Check for entry point
		for _, entry := range []string{"index.html", "app/index.html", "public/index.html"} {
			if _, err := os.Stat(filepath.Join(projectRoot, entry)); err == nil {
				ctx.WriteString(fmt.Sprintf("\nEntry point: %s/%s (open in browser)\n", projectRoot, entry))
				break
			}
		}
	} else {
		ctx.WriteString("\n## Application files: NONE created yet.\n")
		ctx.WriteString(fmt.Sprintf("Project directory: %s\n", projectRoot))
	}

	return ctx.String()
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
	// NOTE: caller must hold chatMu
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
	// NOTE: caller must hold chatMu
	chatPath := filepath.Join(projectRoot, "status", "shaping-chat.json")
	data, _ := json.MarshalIndent(chat, "", "  ")
	tmpPath := chatPath + fmt.Sprintf(".tmp.%d", os.Getpid())
	os.WriteFile(tmpPath, data, 0644)
	os.Rename(tmpPath, chatPath)

	chatSessions[chat.ProjectID] = chat
}

func listAppFiles(projectRoot string) []string {
	var files []string
	appDir := filepath.Join(projectRoot, "app")
	filepath.Walk(appDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(projectRoot, path)
		if info.Size() > 0 { // only list non-empty files
			files = append(files, rel)
		}
		return nil
	})
	if len(files) > 20 {
		files = append(files[:20], fmt.Sprintf("... and %d more files", len(files)-20))
	}
	return files
}

func (s *Server) handleAbortRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	buildMu.Lock()
	cancel, ok := activeBuilds[req.ProjectID]
	buildMu.Unlock()

	if !ok {
		http.Error(w, "no active build for this project", http.StatusNotFound)
		return
	}

	cancel()
	slog.Info("build aborted by user", "project", req.ProjectID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "aborted", "project_id": req.ProjectID})
}

// triggerBuild starts the autonomous build loop for a project.
// Can be called from the chat handler or the build button.
func (s *Server) triggerBuild(projectID, projectRoot string) {
	buildMu.Lock()
	if _, running := activeBuilds[projectID]; running {
		buildMu.Unlock()
		return // already running
	}

	paths := config.DefaultPaths()
	prov := s.pickProvider()
	if prov == nil {
		buildMu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	activeBuilds[projectID] = cancel
	buildMu.Unlock()

	go s.runBuildLoop(ctx, cancel, projectID, projectRoot, paths, prov)
}

func (s *Server) runBuildLoop(ctx context.Context, cancel context.CancelFunc, projectID, projectRoot string, paths config.Paths, prov provider.Provider) {
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
		ContextSize:   s.cfg.Execution.LLMContextSize,
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

	addMCMessage := func(text string) {
		ts := time.Now().UnixMilli()
		chatMu.Lock()
		chat := getOrCreateChat(projectID, "quick")
		chat.Messages = append(chat.Messages, ChatMessage{
			Role:      "assistant",
			Content:   text,
			Timestamp: ts,
		})
		saveChat(projectRoot, chat)
		chatMu.Unlock()

		// Broadcast to frontend so the chat updates in real-time
		s.Broadcast(SSEEvent{
			Type: "mc.message",
			Data: map[string]interface{}{
				"project_id": projectID,
				"role":       "assistant",
				"content":    text,
				"timestamp":  ts,
			},
		})
	}

	addMCMessage("🚀 Build started. I'll report progress as tasks complete.")

	maxCycles := 50
	stuckCount := 0
	taskRetries := make(map[string]int) // track how many times each task has been retried
	const maxTaskRetries = 3
	for cycle := 1; cycle <= maxCycles; cycle++ {
		select {
		case <-ctx.Done():
			addMCMessage("⚠️ Build was cancelled.")
			s.Broadcast(SSEEvent{Type: "build.cancelled", Data: map[string]interface{}{"project_id": projectID}})
			return
		default:
		}

		result := orch.RunProject(ctx, projectRoot)

		taskName := ""
		taskID := ""
		if result.Action.Task != nil {
			taskName = result.Action.Task.Title
			taskID = result.Action.Task.TaskID
		}

		// Only broadcast meaningful milestones — skip noisy/repetitive states
		var milestone string
		broadcast := true
		switch result.StateUpdate {
		case "builder-complete":
			milestone = fmt.Sprintf("✅ Builder completed %s: %s", taskID, taskName)
		case "review-complete":
			milestone = fmt.Sprintf("✅ Reviewer finished %s: %s", taskID, taskName)
		case "done":
			milestone = fmt.Sprintf("🎉 %s closed: %s", taskID, taskName)
		case "builder-failed":
			milestone = fmt.Sprintf("❌ Builder failed on %s: %s", taskID, taskName)
		case "budget-exhausted":
			milestone = "⚠️ Daily budget exhausted — build paused"
		case "rework-reset":
			milestone = fmt.Sprintf("🔁 %s: %s — resetting for retry", taskID, taskName)
		case "blocked-skipped":
			milestone = fmt.Sprintf("⏭️ %s: %s — blocked, skipping", taskID, taskName)
		default:
			// in-flight, rework-reset repeats, and generic cycles are noise — don't show
			broadcast = false
		}

		if broadcast && milestone != "" {
			s.Broadcast(SSEEvent{
				Type: "build.milestone",
				Data: map[string]interface{}{
					"project_id": projectID,
					"cycle":      cycle,
					"action":     result.Action.Kind,
					"state":      result.StateUpdate,
					"task_id":    taskID,
					"task_name":  taskName,
					"milestone":  milestone,
				},
			})
		}

		switch result.StateUpdate {
		case "done":
			addMCMessage(milestone)
			delete(taskRetries, taskID) // clear retries on success
		case "builder-failed":
			addMCMessage(milestone + "\n\nYou can reply here to discuss the issue or click Start Building to retry.")
		case "budget-exhausted":
			addMCMessage(milestone)
		case "rework-reset":
			taskRetries[taskID]++
			if taskRetries[taskID] >= maxTaskRetries {
				addMCMessage(fmt.Sprintf("⚠️ %s has been retried %d times without success. Skipping — the task may need a different approach or a more capable model.", taskID, maxTaskRetries))
				// Mark as blocked so it's skipped
				if result.Action.Task != nil {
					core.SetTaskBlocked(result.Action.Task.Path)
				}
				continue
			}
		}

		if result.Action.Kind == "no-action" {
			break
		}
		if result.StateUpdate == "budget-exhausted" || result.StateUpdate == "builder-failed" {
			break
		}
		if result.Error != nil {
			break
		}
		// Detect stuck cycles — if no real progress for 3 consecutive cycles, stop
		noProgress := result.StateUpdate == "in-flight" || result.StateUpdate == "rework" || result.StateUpdate == "rework-reset" || result.StateUpdate == "blocked-skipped"
		if noProgress {
			stuckCount++
			if stuckCount >= 3 {
				addMCMessage(fmt.Sprintf("⚠️ Build appears stuck on %s (%s). Stopping to avoid wasting resources. You can review the task and try again.",
					taskID, result.StateUpdate))
				break
			}
		} else {
			stuckCount = 0
		}
		// Don't stop on blocked-skipped — continue to try the next task
		if result.StateUpdate == "blocked-skipped" {
			continue
		}
	}

	appFiles := listAppFiles(projectRoot)
	summary := fmt.Sprintf("🏁 Build finished.\n\nProject files are at:\n  %s\n", projectRoot)
	if len(appFiles) > 0 {
		summary += "\nApplication files:\n"
		for _, f := range appFiles {
			summary += fmt.Sprintf("  %s\n", f)
		}
		for _, entry := range []string{"index.html", "app/index.html", "public/index.html"} {
			if _, err := os.Stat(filepath.Join(projectRoot, entry)); err == nil {
				summary += fmt.Sprintf("\nTo open: xdg-open %s/%s\n", projectRoot, entry)
				break
			}
		}
	}
	summary += "\nFiles persist on disk — they stay when Warpspawn is closed."

	addMCMessage(summary)
	s.Broadcast(SSEEvent{
		Type: "build.complete",
		Data: map[string]interface{}{"project_id": projectID, "summary": summary},
	})
}

func (s *Server) handleStartBuild(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		http.Error(w, "project ID required", http.StatusBadRequest)
		return
	}

	// Check for concurrent build
	buildMu.Lock()
	if _, running := activeBuilds[projectID]; running {
		buildMu.Unlock()
		http.Error(w, "build already running for this project", http.StatusConflict)
		return
	}
	buildMu.Unlock()

	paths := config.DefaultPaths()
	projectRoot := filepath.Join(paths.ProjectDir, projectID)
	s.triggerBuild(projectID, projectRoot)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started", "project_id": projectID})
}
