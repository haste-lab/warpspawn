package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/haste-lab/warpspawn/internal/agent"
	"github.com/haste-lab/warpspawn/internal/db"
	"github.com/haste-lab/warpspawn/internal/guard"
	"github.com/haste-lab/warpspawn/internal/provider"
)

// Orchestrator ties together the decision engine, agent executor, guard system, and state management.
type Orchestrator struct {
	Workflow      *Workflow
	Provider      provider.Provider
	DB            *db.DB
	Budget        *guard.Budget
	OnEvent       func(agent.StreamEvent) // callback for UI streaming
	MaxTools      int
	TimeoutS      int
	BuilderModel  string
	ReviewerModel string
	ShellMode     string // unrestricted, restricted, approval
	MaxPromptLen  int    // 0 = unlimited, e.g. 3000 for 4K context models
	ContextSize   int    // num_ctx for Ollama
}

// RunResult is the outcome of a single orchestration cycle for a project.
type OrchestratorResult struct {
	ProjectRoot string
	ProjectID   string
	Action      Action
	AgentResult *agent.RunResult
	StateUpdate string
	Error       error
}

// RunProject executes one orchestration cycle for a project.
func (o *Orchestrator) RunProject(ctx context.Context, projectRoot string) OrchestratorResult {
	projectID := filepath.Base(projectRoot)
	slog.Info("orchestrating project", "project", projectID, "root", projectRoot)

	// Load project state
	state := LoadProjectState(projectRoot)
	if state.Lifecycle != "active" {
		return OrchestratorResult{
			ProjectRoot: projectRoot,
			ProjectID:   projectID,
			StateUpdate: "skipped",
			Error:       fmt.Errorf("project lifecycle is %s, not active", state.Lifecycle),
		}
	}

	// Load tasks and backlog
	tasks := ListTasks(projectRoot)
	backlogText, _ := os.ReadFile(filepath.Join(projectRoot, "backlog", "backlog.md"))
	backlogItems := ParseBacklog(string(backlogText))

	// Load review outcomes for all tasks
	reviewOutcomes := make(map[string]*ReviewOutcome)
	for _, task := range tasks {
		review := LoadLatestReview(projectRoot, task.TaskID)
		if outcome := ClassifyReviewOutcome(review); outcome != nil {
			reviewOutcomes[task.TaskID] = outcome
		}
	}

	// Decision engine
	action := ChooseNextAction(o.Workflow, tasks, backlogItems, reviewOutcomes)
	slog.Info("decision", "action", action.Kind, "rationale", action.Rationale)

	result := OrchestratorResult{
		ProjectRoot: projectRoot,
		ProjectID:   projectID,
		Action:      action,
	}

	// Execute based on action
	switch action.Kind {
	case "handoff-to-builder":
		result = o.executeBuilder(ctx, projectRoot, projectID, action)
	case "manage-in-flight-task":
		result = o.manageInFlight(ctx, projectRoot, projectID, action)
	case "advance-shaping":
		result.StateUpdate = "shaping-needed"
		slog.Info("shaping work needed", "item", action.BacklogItem.ID)
	case "no-action":
		result.StateUpdate = "no-action"
		slog.Info("no actionable work found")
	}

	// Sync project state to SQLite
	if o.DB != nil {
		state = LoadProjectState(projectRoot) // reload after potential changes
		o.DB.UpsertProject(projectID, projectRoot, projectID, state.Lifecycle, state.CurrentStage, state.CurrentEpoch)
		for _, task := range ListTasks(projectRoot) {
			o.DB.UpsertTask(projectID, task.TaskID, task.Path, task.Title, task.Status, task.Priority, task.OwnerRole, task.ModelTier)
		}
	}

	return result
}

func (o *Orchestrator) executeBuilder(ctx context.Context, projectRoot, projectID string, action Action) OrchestratorResult {
	task := action.Task
	if task == nil {
		return OrchestratorResult{ProjectRoot: projectRoot, ProjectID: projectID, Action: action, StateUpdate: "error", Error: fmt.Errorf("no task for builder")}
	}

	// Check budget
	budgetCheck := o.Budget.Check()
	if !budgetCheck.Allowed {
		slog.Warn("budget exhausted", "used", budgetCheck.UsedUSD, "limit", budgetCheck.LimitUSD)
		return OrchestratorResult{ProjectRoot: projectRoot, ProjectID: projectID, Action: action, StateUpdate: "budget-exhausted"}
	}

	// Validate task shape
	if IsPlaceholderSection(task.Objective) || len(task.AcceptanceCriteria) == 0 {
		slog.Warn("task shape invalid", "task", task.TaskID)
		return OrchestratorResult{ProjectRoot: projectRoot, ProjectID: projectID, Action: action, StateUpdate: "task-refused", Error: fmt.Errorf("task missing objective or acceptance criteria")}
	}

	// Infer model tier
	tier := InferModelTier(task)
	slog.Info("builder execution", "task", task.TaskID, "tier", tier.Label)

	// Save pre-execution manifest
	manifest := guard.CreateManifest(projectRoot)
	guard.SaveManifest(projectRoot, manifest)

	// Git pre-execution commit
	gitCommit(projectRoot, fmt.Sprintf("[warpspawn] pre-execution: Builder %s", task.TaskID))

	// Update task status to in-build
	updateTaskStatusInFile(task.Path, "in-build")
	updateProjectStage(projectRoot, "in progress")

	// Build prompt
	systemPrompt, userPrompt := BuildBuilderPrompt(task)

	// Run agent
	startTime := time.Now()
	agentResult := agent.Run(ctx, agent.RunConfig{
		ProjectRoot:    projectRoot,
		Model:          o.BuilderModel,
		Provider:       o.Provider,
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		MaxToolCalls:   o.MaxTools,
		AgentTimeout:   time.Duration(o.TimeoutS) * time.Second,
		ShellMode:      agent.ShellMode(o.ShellMode),
		MaxPromptLen:   o.MaxPromptLen,
		CommandTimeout: 30 * time.Second,
		ContextSize:   o.ContextSize,
		OnChunk:       o.OnEvent,
	})
	duration := time.Since(startTime)

	// Record budget usage
	o.Budget.Record(guard.BudgetEntry{
		Project:      projectID,
		Role:         "builder",
		TaskID:       task.TaskID,
		Model:        agentResult.TotalUsage.Model,
		InputTokens:  agentResult.TotalUsage.InputTokens,
		OutputTokens: agentResult.TotalUsage.OutputTokens,
	})

	// Record run in database
	if o.DB != nil {
		status := "completed"
		if !agentResult.Success {
			status = "failed"
		}
		o.DB.RecordRun(db.RunRecord{
			ProjectID:    projectID,
			TaskID:       task.TaskID,
			Role:         "builder",
			Model:        agentResult.TotalUsage.Model,
			Provider:     o.Provider.ID(),
			InputTokens:  agentResult.TotalUsage.InputTokens,
			OutputTokens: agentResult.TotalUsage.OutputTokens,
			CostUSD:      guard.CalculateCost(agentResult.TotalUsage.Model, agentResult.TotalUsage.InputTokens, agentResult.TotalUsage.OutputTokens),
			ToolCalls:    agentResult.ToolCalls,
			DurationMS:   duration.Milliseconds(),
			Status:       status,
			Summary:      truncate(agentResult.Summary, 500),
			StartedAt:    startTime.UTC().Format(time.RFC3339),
			CompletedAt:  time.Now().UTC().Format(time.RFC3339),
		})
	}

	// Post-execution validation
	caps := DefaultRoleCapabilities["builder"]
	validation := guard.ValidateRoleChanges(projectRoot, manifest, caps.MayEdit)
	if len(validation.Violations) > 0 {
		slog.Warn("role boundary violations", "violations", len(validation.Violations))
		guard.RevertUnauthorizedFiles(projectRoot, validation.Violations)
	}
	guard.ArchiveManifest(projectRoot)

	// Git post-execution commit
	gitCommit(projectRoot, fmt.Sprintf("[warpspawn] post-execution: Builder %s — %s", task.TaskID, status(agentResult.Success)))

	slog.Info("builder finished",
		"task", task.TaskID,
		"success", agentResult.Success,
		"tools", agentResult.ToolCalls,
		"input_tokens", agentResult.TotalUsage.InputTokens,
		"output_tokens", agentResult.TotalUsage.OutputTokens,
		"duration", duration.Round(time.Second),
	)

	stateUpdate := "in-progress"
	if agentResult.Success {
		// Builder completed — advance task to in-review for Reviewer pickup
		// Re-read the task file to check if Builder updated it
		updatedTask, _ := ParseTaskFile(task.Path)
		if updatedTask.Status != "in-review" {
			// Builder didn't update the task status — do it automatically
			updateTaskStatusInFile(task.Path, "in-review")
			slog.Info("auto-advanced task to in-review", "task", task.TaskID)
		}

		// Write a minimal handoff if Builder didn't
		if IsPlaceholderSection(updatedTask.Validation) {
			appendToTaskSection(task.Path, "Validation", fmt.Sprintf("Builder completed with %d tool calls. Summary: %s", agentResult.ToolCalls, truncate(agentResult.Summary, 200)))
		}
		if IsPlaceholderSection(updatedTask.ImplementationNotes) {
			appendToTaskSection(task.Path, "Implementation Notes", fmt.Sprintf("Auto-generated: Builder executed successfully. %s", truncate(agentResult.Summary, 200)))
		}

		// Post-build validation: check that Builder actually created files with content
		sourceFiles := task.SourceFiles
		emptyFiles := 0
		for _, sf := range sourceFiles {
			fullPath := filepath.Join(projectRoot, sf)
			info, err := os.Stat(fullPath)
			if err != nil || (info != nil && !info.IsDir() && info.Size() == 0) {
				emptyFiles++
			}
		}
		if emptyFiles > 0 && len(sourceFiles) > 0 {
			slog.Warn("post-build validation: empty or missing source files", "task", task.TaskID, "empty", emptyFiles, "total", len(sourceFiles))
		}

		updateProjectStage(projectRoot, "in review")
		stateUpdate = "builder-complete"
	} else {
		stateUpdate = "builder-failed"
		updateTaskStatusInFile(task.Path, "blocked")
		updateProjectStage(projectRoot, "blocked")
	}

	return OrchestratorResult{
		ProjectRoot: projectRoot,
		ProjectID:   projectID,
		Action:      action,
		AgentResult: &agentResult,
		StateUpdate: stateUpdate,
	}
}

func (o *Orchestrator) manageInFlight(ctx context.Context, projectRoot, projectID string, action Action) OrchestratorResult {
	task := action.Task
	if task == nil {
		return OrchestratorResult{ProjectRoot: projectRoot, ProjectID: projectID, Action: action, StateUpdate: "error"}
	}

	// Check if review outcome exists — but skip if task is already in rework
	// (the old review file still says "rejected", which would re-trigger the
	// rejected handler and prevent the rework-reset handler from ever running)
	if action.ReviewOutcome != nil && task.Status != "rework" {
		switch action.ReviewOutcome.Kind {
		case "approved":
			slog.Info("task approved by reviewer", "task", task.TaskID)
			updateTaskStatusInFile(task.Path, "done")
			updateProjectStage(projectRoot, "done")
			gitCommit(projectRoot, fmt.Sprintf("[warpspawn] closed: %s — approved", task.TaskID))
			return OrchestratorResult{ProjectRoot: projectRoot, ProjectID: projectID, Action: action, StateUpdate: "done"}

		case "rejected":
			slog.Info("task rejected by reviewer", "task", task.TaskID)
			updateTaskStatusInFile(task.Path, "rework")
			updateProjectStage(projectRoot, "rework")
			return OrchestratorResult{ProjectRoot: projectRoot, ProjectID: projectID, Action: action, StateUpdate: "rework"}

		case "blocked":
			slog.Info("task blocked by reviewer", "task", task.TaskID)
			updateTaskStatusInFile(task.Path, "blocked")
			updateProjectStage(projectRoot, "blocked")
			return OrchestratorResult{ProjectRoot: projectRoot, ProjectID: projectID, Action: action, StateUpdate: "blocked"}
		}
	}

	// Check if builder has completed evidence → route to reviewer
	if action.BuilderOutcome != nil && action.BuilderOutcome.Kind == "review-ready" {
		slog.Info("builder evidence complete, routing to reviewer", "task", task.TaskID)
		return o.executeReviewer(ctx, projectRoot, projectID, action)
	}

	// Handle blocked tasks — skip them so the build loop doesn't spin
	if task.Status == "blocked" {
		slog.Info("task is blocked, skipping", "task", task.TaskID)
		return OrchestratorResult{ProjectRoot: projectRoot, ProjectID: projectID, Action: action, StateUpdate: "blocked-skipped"}
	}

	// Handle in-review tasks without review — try running the reviewer
	if task.Status == "in-review" && action.ReviewOutcome == nil {
		slog.Info("task in-review without review, attempting reviewer", "task", task.TaskID)
		return o.executeReviewer(ctx, projectRoot, projectID, action)
	}

	// Handle rework tasks — reset to ready-for-build and remove stale review
	if task.Status == "rework" {
		slog.Info("task in rework, resetting to ready-for-build", "task", task.TaskID)
		updateTaskStatusInFile(task.Path, "ready-for-build")
		// Remove the old review so the Reviewer starts fresh after rebuild
		removeReviewsForTask(projectRoot, task.TaskID)
		return OrchestratorResult{ProjectRoot: projectRoot, ProjectID: projectID, Action: action, StateUpdate: "rework-reset"}
	}

	// In-flight but no clear next step
	slog.Info("in-flight task, no state change", "task", task.TaskID, "status", task.Status)
	return OrchestratorResult{ProjectRoot: projectRoot, ProjectID: projectID, Action: action, StateUpdate: "in-flight"}
}

func (o *Orchestrator) executeReviewer(ctx context.Context, projectRoot, projectID string, action Action) OrchestratorResult {
	task := action.Task

	// Check budget
	budgetCheck := o.Budget.Check()
	if !budgetCheck.Allowed {
		return OrchestratorResult{ProjectRoot: projectRoot, ProjectID: projectID, Action: action, StateUpdate: "budget-exhausted"}
	}

	slog.Info("reviewer execution", "task", task.TaskID)

	// Build prompt
	systemPrompt, userPrompt := BuildReviewerPrompt(task)

	startTime := time.Now()
	agentResult := agent.Run(ctx, agent.RunConfig{
		ProjectRoot:    projectRoot,
		Model:          o.ReviewerModel,
		Provider:       o.Provider,
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		MaxToolCalls:   o.MaxTools,
		AgentTimeout:   time.Duration(o.TimeoutS) * time.Second,
		ShellMode:      agent.ShellMode(o.ShellMode),
		MaxPromptLen:   o.MaxPromptLen,
		CommandTimeout: 30 * time.Second,
		ContextSize:   o.ContextSize,
		OnChunk:       o.OnEvent,
	})
	duration := time.Since(startTime)

	// Record budget
	o.Budget.Record(guard.BudgetEntry{
		Project:      projectID,
		Role:         "reviewer-qa",
		TaskID:       task.TaskID,
		Model:        agentResult.TotalUsage.Model,
		InputTokens:  agentResult.TotalUsage.InputTokens,
		OutputTokens: agentResult.TotalUsage.OutputTokens,
	})

	// Record run
	if o.DB != nil {
		runStatus := "completed"
		if !agentResult.Success {
			runStatus = "failed"
		}
		o.DB.RecordRun(db.RunRecord{
			ProjectID:    projectID,
			TaskID:       task.TaskID,
			Role:         "reviewer-qa",
			Model:        agentResult.TotalUsage.Model,
			Provider:     o.Provider.ID(),
			InputTokens:  agentResult.TotalUsage.InputTokens,
			OutputTokens: agentResult.TotalUsage.OutputTokens,
			CostUSD:      guard.CalculateCost(agentResult.TotalUsage.Model, agentResult.TotalUsage.InputTokens, agentResult.TotalUsage.OutputTokens),
			ToolCalls:    agentResult.ToolCalls,
			DurationMS:   duration.Milliseconds(),
			Status:       runStatus,
			Summary:      truncate(agentResult.Summary, 500),
			StartedAt:    startTime.UTC().Format(time.RFC3339),
			CompletedAt:  time.Now().UTC().Format(time.RFC3339),
		})
	}

	// If Reviewer succeeded but didn't write a review file, auto-generate one
	if agentResult.Success {
		reviewFile := LoadLatestReview(projectRoot, task.TaskID)
		if reviewFile == nil {
			// Reviewer used task_complete without writing a review artifact — generate one
			reviewID := fmt.Sprintf("REVIEW-%s-%s", task.TaskID, time.Now().Format("2006-01-02"))
			reviewContent := fmt.Sprintf(`# Review Report

## Metadata
- Review ID: %s
- Task ID: %s
- Reviewer: Reviewer/QA (auto-generated from agent output)
- Outcome: approved
- Date: %s

## Acceptance Criteria Result
Agent reported: %s

## Defects
None reported.

## Final Recommendation
Approved based on agent verification.
`, reviewID, task.TaskID, time.Now().Format("2006-01-02"), truncate(agentResult.Summary, 300))
			reviewPath := filepath.Join(projectRoot, "reviews", strings.ToLower(reviewID)+".md")
			os.MkdirAll(filepath.Dir(reviewPath), 0755)
			tmpPath := reviewPath + fmt.Sprintf(".tmp.%d", os.Getpid())
			os.WriteFile(tmpPath, []byte(reviewContent), 0644)
			os.Rename(tmpPath, reviewPath)
			slog.Info("auto-generated review artifact", "task", task.TaskID, "review", reviewID)
		}
	}

	gitCommit(projectRoot, fmt.Sprintf("[warpspawn] post-execution: Reviewer %s — %s", task.TaskID, status(agentResult.Success)))

	slog.Info("reviewer finished",
		"task", task.TaskID,
		"success", agentResult.Success,
		"tools", agentResult.ToolCalls,
		"duration", duration.Round(time.Second),
	)

	return OrchestratorResult{
		ProjectRoot: projectRoot,
		ProjectID:   projectID,
		Action:      action,
		AgentResult: &agentResult,
		StateUpdate: "review-complete",
	}
}

// helper functions

func updateTaskStatusInFile(taskPath, newStatus string) {
	data, err := os.ReadFile(taskPath)
	if err != nil {
		slog.Warn("failed to read task file for status update", "path", taskPath, "error", err)
		return
	}
	text := string(data)
	updated := strings.Replace(text, "- Status: "+ExtractMetadataValue(text, "Status"), "- Status: "+newStatus, 1)
	if err := atomicWrite(taskPath, []byte(updated)); err != nil {
		slog.Error("failed to update task status", "path", taskPath, "error", err)
	}
}

func appendToTaskSection(taskPath, section, content string) {
	data, err := os.ReadFile(taskPath)
	if err != nil {
		slog.Warn("failed to read task file for section update", "path", taskPath, "error", err)
		return
	}
	text := string(data)
	placeholder := "## " + section + "\npending"
	replacement := "## " + section + "\n" + content
	if strings.Contains(text, placeholder) {
		updated := strings.Replace(text, placeholder, replacement, 1)
		if err := atomicWrite(taskPath, []byte(updated)); err != nil {
			slog.Error("failed to update task section", "path", taskPath, "section", section, "error", err)
		}
	}
}

func updateProjectStage(projectRoot, stage string) {
	statePath := filepath.Join(projectRoot, "status", "project-state.md")
	data, err := os.ReadFile(statePath)
	if err != nil {
		slog.Warn("failed to read project state", "path", statePath, "error", err)
		return
	}
	text := string(data)
	current := ExtractMetadataValue(text, "Current Stage")
	if current != "" {
		updated := strings.Replace(text, "- Current Stage: "+current, "- Current Stage: "+stage, 1)
		if err := atomicWrite(statePath, []byte(updated)); err != nil {
			slog.Error("failed to update project stage", "path", statePath, "error", err)
		}
	}
}

// SetTaskBlocked marks a task as blocked in its file.
func SetTaskBlocked(taskPath string) {
	updateTaskStatusInFile(taskPath, "blocked")
}

// removeReviewsForTask deletes review files for a task so the Reviewer starts fresh after rework.
func removeReviewsForTask(projectRoot, taskID string) {
	reviewsDir := filepath.Join(projectRoot, "reviews")
	entries, err := os.ReadDir(reviewsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(strings.ToUpper(name), strings.ToUpper(taskID)) && strings.HasSuffix(name, ".md") && name != "README.md" {
			os.Remove(filepath.Join(reviewsDir, name))
			slog.Debug("removed stale review", "file", name, "task", taskID)
		}
	}
}

// atomicWrite writes data to a file using the write-to-temp-then-rename pattern.
func atomicWrite(path string, data []byte) error {
	tmpPath := path + fmt.Sprintf(".tmp.%d", os.Getpid())
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func gitCommit(projectRoot, message string) {
	// Check if git is available and the project has a .git dir
	gitDir := filepath.Join(projectRoot, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return
	}

	// Stage all changes
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = projectRoot
	cmd.Run()

	// Commit (only if there are staged changes — no empty commits)
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = projectRoot
	cmd.Run()
}

func status(success bool) string {
	if success {
		return "completed"
	}
	return "failed"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
