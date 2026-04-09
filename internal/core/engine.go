package core

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

// Action represents the decision engine's chosen next action.
type Action struct {
	Kind       string // manage-in-flight, handoff-to-builder, advance-shaping, no-action
	Rationale  string
	Task       *Task
	BacklogItem *BacklogItem
	BuilderOutcome *BuilderOutcome
	ReviewOutcome  *ReviewOutcome
	UnrecognizedStatuses []UnrecognizedStatus
}

// BuilderOutcome classifies what the Builder left behind.
type BuilderOutcome struct {
	Kind string // review-ready, blocked, incomplete
	Note string
}

// ReviewOutcome classifies a review result.
type ReviewOutcome struct {
	Kind   string // approved, rejected, blocked
	Note   string
	Review *Review
}

// UnrecognizedStatus flags a task with a status the workflow doesn't know.
type UnrecognizedStatus struct {
	TaskID string
	Status string
}

// ClassifyBuilderOutcome determines if a Builder task has completion evidence.
func ClassifyBuilderOutcome(task *Task) *BuilderOutcome {
	if task.OwnerRole != "Builder" {
		return nil
	}

	validationPresent := !IsPlaceholderSection(task.Validation)
	implementationPresent := !IsPlaceholderSection(task.ImplementationNotes)
	handoffPresent := !IsPlaceholderSection(task.Handoff)

	if task.Status == "in-review" && validationPresent && implementationPresent && handoffPresent {
		return &BuilderOutcome{
			Kind: "review-ready",
			Note: fmt.Sprintf("Builder evidence is complete for %s; task is ready for review.", task.TaskID),
		}
	}

	if task.Status == "blocked" && handoffPresent {
		return &BuilderOutcome{
			Kind: "blocked",
			Note: fmt.Sprintf("Builder marked %s as blocked with handoff context.", task.TaskID),
		}
	}

	return &BuilderOutcome{
		Kind: "incomplete",
		Note: fmt.Sprintf("Builder output for %s is not yet complete enough for ingestion.", task.TaskID),
	}
}

// ClassifyReviewOutcome determines the review result for a task.
func ClassifyReviewOutcome(review *Review) *ReviewOutcome {
	if review == nil || review.Outcome == "" {
		return nil
	}

	switch review.Outcome {
	case "approved":
		return &ReviewOutcome{Kind: "approved", Note: fmt.Sprintf("Reviewer approved %s.", review.TaskID), Review: review}
	case "rejected":
		return &ReviewOutcome{Kind: "rejected", Note: fmt.Sprintf("Reviewer rejected %s.", review.TaskID), Review: review}
	case "blocked":
		return &ReviewOutcome{Kind: "blocked", Note: fmt.Sprintf("Reviewer blocked %s.", review.TaskID), Review: review}
	}
	return nil
}

// ChooseNextAction is the core decision engine.
// It reads project state and determines the highest-priority next step.
func ChooseNextAction(workflow *Workflow, tasks []Task, backlogItems []BacklogItem, reviewOutcomes map[string]*ReviewOutcome) Action {
	// Normalize statuses and collect unrecognized ones
	var unrecognized []UnrecognizedStatus
	for i := range tasks {
		normalized := NormalizeStatus(tasks[i].Status)
		if normalized != tasks[i].Status {
			slog.Debug("normalized task status", "task", tasks[i].TaskID, "from", tasks[i].Status, "to", normalized)
			tasks[i].Status = normalized
		}
		if !workflow.IsKnownStatus(tasks[i].Status) {
			unrecognized = append(unrecognized, UnrecognizedStatus{TaskID: tasks[i].TaskID, Status: tasks[i].Status})
		}
	}

	// 1. Look for in-flight work (highest priority: preserve flow)
	for i := range tasks {
		if workflow.IsActive(tasks[i].Status) {
			task := &tasks[i]
			return Action{
				Kind:                 "manage-in-flight-task",
				Rationale:            fmt.Sprintf("Preserve flow on in-flight work before opening new work (%s).", task.TaskID),
				Task:                 task,
				BuilderOutcome:       ClassifyBuilderOutcome(task),
				ReviewOutcome:        reviewOutcomes[task.TaskID],
				UnrecognizedStatuses: unrecognized,
			}
		}
	}

	// 2. Look for ready-for-build tasks
	readyTasks := filterAndSortByPriority(tasks, workflow.Routing.ReadyStatus)
	if len(readyTasks) > 0 {
		task := &readyTasks[0]
		return Action{
			Kind:                 "handoff-to-builder",
			Rationale:            fmt.Sprintf("Selected highest-priority ready task %s.", task.TaskID),
			Task:                 task,
			ReviewOutcome:        reviewOutcomes[task.TaskID],
			UnrecognizedStatuses: unrecognized,
		}
	}

	// 3. Look for shaping work
	shapingItems := filterBacklogByStatus(backlogItems, workflow.Routing.ShapingStatus)
	if len(shapingItems) > 0 {
		item := &shapingItems[0]
		return Action{
			Kind:                 "advance-shaping",
			Rationale:            fmt.Sprintf("No ready tasks exist, so shaping work %s is next.", item.ID),
			BacklogItem:          item,
			UnrecognizedStatuses: unrecognized,
		}
	}

	// 4. No actionable work
	unrecognizedNote := ""
	if len(unrecognized) > 0 {
		names := make([]string, len(unrecognized))
		for i, u := range unrecognized {
			names[i] = fmt.Sprintf("%s=%s", u.TaskID, u.Status)
		}
		unrecognizedNote = fmt.Sprintf(" Warning: %d task(s) have unrecognized statuses: %s.", len(unrecognized), strings.Join(names, ", "))
	}

	return Action{
		Kind:                 "no-action",
		Rationale:            fmt.Sprintf("No ready tasks or shaping items were found. Manual review is required.%s", unrecognizedNote),
		UnrecognizedStatuses: unrecognized,
	}
}

func filterAndSortByPriority(tasks []Task, status string) []Task {
	var filtered []Task
	for _, t := range tasks {
		if t.Status == status {
			filtered = append(filtered, t)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		pi, pj := PriorityRank(filtered[i].Priority), PriorityRank(filtered[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return filtered[i].TaskID < filtered[j].TaskID
	})
	return filtered
}

func filterBacklogByStatus(items []BacklogItem, status string) []BacklogItem {
	var filtered []BacklogItem
	for _, item := range items {
		if item.Status == status {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		pi, pj := PriorityRank(filtered[i].Priority), PriorityRank(filtered[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return filtered[i].ID < filtered[j].ID
	})
	return filtered
}

