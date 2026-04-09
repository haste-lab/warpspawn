package core

import "testing"

func makeTask(id, status, priority, ownerRole string) Task {
	return Task{
		TaskID:    id,
		Status:    status,
		Priority:  priority,
		OwnerRole: ownerRole,
	}
}

func TestChooseNextAction_InFlightTask(t *testing.T) {
	w := DefaultWorkflow
	tasks := []Task{
		makeTask("TASK-001", "in-build", "P0", "Builder"),
		makeTask("TASK-002", "ready-for-build", "P0", "Builder"),
	}

	action := ChooseNextAction(&w, tasks, nil, nil)
	if action.Kind != "manage-in-flight-task" {
		t.Errorf("expected manage-in-flight-task, got %s", action.Kind)
	}
	if action.Task.TaskID != "TASK-001" {
		t.Errorf("expected TASK-001, got %s", action.Task.TaskID)
	}
}

func TestChooseNextAction_ReadyForBuild(t *testing.T) {
	w := DefaultWorkflow
	tasks := []Task{
		makeTask("TASK-002", "ready-for-build", "P1", "Builder"),
		makeTask("TASK-001", "ready-for-build", "P0", "Builder"),
		makeTask("TASK-003", "done", "P0", "Builder"),
	}

	action := ChooseNextAction(&w, tasks, nil, nil)
	if action.Kind != "handoff-to-builder" {
		t.Errorf("expected handoff-to-builder, got %s", action.Kind)
	}
	// Should pick P0 first, then by ID
	if action.Task.TaskID != "TASK-001" {
		t.Errorf("expected TASK-001 (P0), got %s", action.Task.TaskID)
	}
}

func TestChooseNextAction_AdvanceShaping(t *testing.T) {
	w := DefaultWorkflow
	tasks := []Task{
		makeTask("TASK-001", "done", "P0", "Builder"),
	}
	backlog := []BacklogItem{
		{ID: "ITEM-002", Status: "shaping", Priority: "P1"},
		{ID: "ITEM-001", Status: "shaping", Priority: "P0"},
	}

	action := ChooseNextAction(&w, tasks, backlog, nil)
	if action.Kind != "advance-shaping" {
		t.Errorf("expected advance-shaping, got %s", action.Kind)
	}
	if action.BacklogItem.ID != "ITEM-001" {
		t.Errorf("expected ITEM-001 (P0), got %s", action.BacklogItem.ID)
	}
}

func TestChooseNextAction_NoAction(t *testing.T) {
	w := DefaultWorkflow
	tasks := []Task{
		makeTask("TASK-001", "done", "P0", "Builder"),
	}

	action := ChooseNextAction(&w, tasks, nil, nil)
	if action.Kind != "no-action" {
		t.Errorf("expected no-action, got %s", action.Kind)
	}
}

func TestChooseNextAction_StatusNormalization(t *testing.T) {
	w := DefaultWorkflow
	tasks := []Task{
		makeTask("TASK-001", "ready-for-review", "P0", "Builder"),
	}

	action := ChooseNextAction(&w, tasks, nil, nil)
	// "ready-for-review" normalizes to "in-review", which is active
	if action.Kind != "manage-in-flight-task" {
		t.Errorf("expected manage-in-flight-task after normalization, got %s", action.Kind)
	}
}

func TestChooseNextAction_UnrecognizedStatus(t *testing.T) {
	w := DefaultWorkflow
	tasks := []Task{
		makeTask("TASK-001", "totally-unknown", "P0", "Builder"),
	}

	action := ChooseNextAction(&w, tasks, nil, nil)
	if action.Kind != "no-action" {
		t.Errorf("expected no-action for unknown status, got %s", action.Kind)
	}
	if len(action.UnrecognizedStatuses) != 1 {
		t.Errorf("expected 1 unrecognized status, got %d", len(action.UnrecognizedStatuses))
	}
	if action.UnrecognizedStatuses[0].TaskID != "TASK-001" {
		t.Errorf("expected TASK-001, got %s", action.UnrecognizedStatuses[0].TaskID)
	}
}

func TestChooseNextAction_ReviewOutcomeApproved(t *testing.T) {
	w := DefaultWorkflow
	tasks := []Task{
		{
			TaskID:              "TASK-001",
			Status:              "in-review",
			Priority:            "P0",
			OwnerRole:           "Builder",
			Validation:          "passed all checks",
			ImplementationNotes: "built the thing",
			Handoff:             "ready for review",
		},
	}
	reviews := map[string]*ReviewOutcome{
		"TASK-001": {Kind: "approved", Note: "Approved", Review: &Review{TaskID: "TASK-001", Outcome: "approved"}},
	}

	action := ChooseNextAction(&w, tasks, nil, reviews)
	if action.Kind != "manage-in-flight-task" {
		t.Errorf("expected manage-in-flight-task, got %s", action.Kind)
	}
	if action.ReviewOutcome == nil || action.ReviewOutcome.Kind != "approved" {
		t.Error("expected approved review outcome")
	}
	if action.BuilderOutcome == nil || action.BuilderOutcome.Kind != "review-ready" {
		t.Errorf("expected review-ready builder outcome, got %v", action.BuilderOutcome)
	}
}

func TestClassifyBuilderOutcome_ReviewReady(t *testing.T) {
	task := &Task{
		TaskID:              "TASK-001",
		Status:              "in-review",
		OwnerRole:           "Builder",
		Validation:          "tested successfully",
		ImplementationNotes: "implemented feature X",
		Handoff:             "ready for review",
	}
	outcome := ClassifyBuilderOutcome(task)
	if outcome.Kind != "review-ready" {
		t.Errorf("expected review-ready, got %s", outcome.Kind)
	}
}

func TestClassifyBuilderOutcome_Incomplete(t *testing.T) {
	task := &Task{
		TaskID:    "TASK-001",
		Status:    "in-build",
		OwnerRole: "Builder",
	}
	outcome := ClassifyBuilderOutcome(task)
	if outcome.Kind != "incomplete" {
		t.Errorf("expected incomplete, got %s", outcome.Kind)
	}
}

func TestClassifyReviewOutcome(t *testing.T) {
	tests := []struct {
		outcome  string
		expected string
	}{
		{"approved", "approved"},
		{"rejected", "rejected"},
		{"blocked", "blocked"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.outcome, func(t *testing.T) {
			review := &Review{TaskID: "T1", Outcome: tt.outcome}
			if tt.outcome == "" {
				review = nil
			}
			result := ClassifyReviewOutcome(review)
			if tt.expected == "" {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if result == nil || result.Kind != tt.expected {
				t.Errorf("expected %s, got %v", tt.expected, result)
			}
		})
	}
}
