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
	BuilderModel  string // model for Builder agent
	ReviewerModel string // model for Reviewer agent
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
		ProjectRoot:  projectRoot,
		Model:        o.BuilderModel,
		Provider:     o.Provider,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxToolCalls: o.MaxTools,
		OnChunk:      o.OnEvent,
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
	if !agentResult.Success {
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

	// Check if review outcome exists
	if action.ReviewOutcome != nil {
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
		ProjectRoot:  projectRoot,
		Model:        o.ReviewerModel,
		Provider:     o.Provider,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxToolCalls: o.MaxTools,
		OnChunk:      o.OnEvent,
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
		return
	}
	text := string(data)
	updated := strings.Replace(text, "- Status: "+ExtractMetadataValue(text, "Status"), "- Status: "+newStatus, 1)
	tmpPath := taskPath + fmt.Sprintf(".tmp.%d", os.Getpid())
	os.WriteFile(tmpPath, []byte(updated), 0644)
	os.Rename(tmpPath, taskPath)
}

func updateProjectStage(projectRoot, stage string) {
	statePath := filepath.Join(projectRoot, "status", "project-state.md")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return
	}
	text := string(data)
	current := ExtractMetadataValue(text, "Current Stage")
	if current != "" {
		updated := strings.Replace(text, "- Current Stage: "+current, "- Current Stage: "+stage, 1)
		tmpPath := statePath + fmt.Sprintf(".tmp.%d", os.Getpid())
		os.WriteFile(tmpPath, []byte(updated), 0644)
		os.Rename(tmpPath, statePath)
	}
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

	// Commit
	cmd = exec.Command("git", "commit", "-m", message, "--allow-empty")
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
