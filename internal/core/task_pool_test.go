package core

import (
	"errors"
	"testing"
)

func TestTaskPool_AddAndGet(t *testing.T) {
	pool := NewTaskPool()
	task := NewTask("Test task")
	pool.AddTask(task)

	got := pool.GetTask(task.ID)
	if got == nil {
		t.Fatal("Expected to get task back")
	}
	if got.Text != task.Text {
		t.Errorf("Expected %q, got %q", task.Text, got.Text)
	}
}

func TestTaskPool_RemoveTask(t *testing.T) {
	pool := NewTaskPool()
	task := NewTask("Test")
	pool.AddTask(task)
	pool.RemoveTask(task.ID)

	if pool.GetTask(task.ID) != nil {
		t.Error("Expected task to be removed")
	}
	if pool.Count() != 0 {
		t.Errorf("Expected 0 tasks, got %d", pool.Count())
	}
}

func TestTaskPool_GetTasksByStatus(t *testing.T) {
	pool := NewTaskPool()
	t1 := NewTask("Todo task")
	t2 := NewTask("Blocked task")
	_ = t2.UpdateStatus(StatusBlocked)
	pool.AddTask(t1)
	pool.AddTask(t2)

	todos := pool.GetTasksByStatus(StatusTodo)
	if len(todos) != 1 {
		t.Errorf("Expected 1 todo task, got %d", len(todos))
	}

	blocked := pool.GetTasksByStatus(StatusBlocked)
	if len(blocked) != 1 {
		t.Errorf("Expected 1 blocked task, got %d", len(blocked))
	}
}

func TestTaskPool_GetAvailableForDoors(t *testing.T) {
	pool := NewTaskPool()
	for i := 0; i < 5; i++ {
		pool.AddTask(NewTask("Task"))
	}

	available := pool.GetAvailableForDoors()
	if len(available) != 5 {
		t.Errorf("Expected 5 available tasks, got %d", len(available))
	}

	// Complete one task
	allTasks := pool.GetAllTasks()
	_ = allTasks[0].UpdateStatus(StatusComplete)
	pool.UpdateTask(allTasks[0])

	available = pool.GetAvailableForDoors()
	if len(available) != 4 {
		t.Errorf("Expected 4 available tasks after completing one, got %d", len(available))
	}
}

func TestTaskPool_RecentlyShown(t *testing.T) {
	pool := NewTaskPool()
	task := NewTask("Test")
	pool.AddTask(task)

	if pool.IsRecentlyShown(task.ID) {
		t.Error("Task should not be recently shown initially")
	}

	pool.MarkRecentlyShown(task.ID)
	if !pool.IsRecentlyShown(task.ID) {
		t.Error("Task should be recently shown after marking")
	}
}

func TestTaskPool_GetAvailableForDoors_FewTasks(t *testing.T) {
	pool := NewTaskPool()
	t1 := NewTask("Only task")
	pool.AddTask(t1)
	pool.MarkRecentlyShown(t1.ID)

	// With < 3 tasks, should include recently shown
	available := pool.GetAvailableForDoors()
	if len(available) != 1 {
		t.Errorf("Expected 1 available task (including recently shown), got %d", len(available))
	}
}

func TestTaskPool_FindBySourceRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		refs     []SourceRef
		provider string
		nativeID string
		wantNil  bool
	}{
		{
			name:     "finds task by exact match",
			refs:     []SourceRef{{Provider: "jira", NativeID: "PROJ-42"}},
			provider: "jira",
			nativeID: "PROJ-42",
			wantNil:  false,
		},
		{
			name:     "returns nil for missing ref",
			refs:     []SourceRef{{Provider: "jira", NativeID: "PROJ-42"}},
			provider: "jira",
			nativeID: "PROJ-99",
			wantNil:  true,
		},
		{
			name:     "returns nil for different provider",
			refs:     []SourceRef{{Provider: "jira", NativeID: "PROJ-42"}},
			provider: "obsidian",
			nativeID: "PROJ-42",
			wantNil:  true,
		},
		{
			name: "finds via second ref",
			refs: []SourceRef{
				{Provider: "textfile", NativeID: "abc"},
				{Provider: "jira", NativeID: "PROJ-42"},
			},
			provider: "jira",
			nativeID: "PROJ-42",
			wantNil:  false,
		},
		{
			name:     "empty pool returns nil",
			refs:     nil,
			provider: "jira",
			nativeID: "PROJ-42",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pool := NewTaskPool()
			if len(tt.refs) > 0 {
				task := NewTask("test task")
				task.SourceRefs = tt.refs
				pool.AddTask(task)
			}

			got := pool.FindBySourceRef(tt.provider, tt.nativeID)
			if tt.wantNil && got != nil {
				t.Errorf("FindBySourceRef(%q, %q) = %v, want nil", tt.provider, tt.nativeID, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("FindBySourceRef(%q, %q) = nil, want task", tt.provider, tt.nativeID)
			}
		})
	}
}

func TestTaskPool_FindBySourceRef_IndexUpdatedOnUpdate(t *testing.T) {
	t.Parallel()

	pool := NewTaskPool()
	task := NewTask("task")
	task.AddSourceRef("jira", "PROJ-1")
	pool.AddTask(task)

	if pool.FindBySourceRef("jira", "PROJ-1") == nil {
		t.Fatal("expected to find task after add")
	}

	// Update task with new ref, removing old
	task.SourceRefs = []SourceRef{{Provider: "obsidian", NativeID: "note-1"}}
	pool.UpdateTask(task)

	if pool.FindBySourceRef("jira", "PROJ-1") != nil {
		t.Error("old ref should be removed from index after update")
	}
	if pool.FindBySourceRef("obsidian", "note-1") == nil {
		t.Error("new ref should be in index after update")
	}
}

func TestTaskPool_FindBySourceRef_IndexCleanedOnRemove(t *testing.T) {
	t.Parallel()

	pool := NewTaskPool()
	task := NewTask("task")
	task.AddSourceRef("jira", "PROJ-1")
	pool.AddTask(task)

	pool.RemoveTask(task.ID)

	if pool.FindBySourceRef("jira", "PROJ-1") != nil {
		t.Error("ref should be removed from index after task removal")
	}
}

func TestTaskPool_FindBySourceRef_NoRefsTask(t *testing.T) {
	t.Parallel()

	pool := NewTaskPool()
	task := NewTask("no refs task")
	pool.AddTask(task)

	// Should not panic, just return nil for any lookup
	if pool.FindBySourceRef("any", "any") != nil {
		t.Error("expected nil for task with no source refs")
	}
}

func TestTaskPool_FindByPrefix(t *testing.T) {
	t.Parallel()

	pool := NewTaskPool()
	t1 := &Task{ID: "abc12345-0000-0000-0000-000000000000", Text: "First task", Status: StatusTodo}
	t2 := &Task{ID: "abc12345-1111-1111-1111-111111111111", Text: "Second task", Status: StatusTodo}
	t3 := &Task{ID: "def67890-0000-0000-0000-000000000000", Text: "Third task", Status: StatusTodo}
	pool.AddTask(t1)
	pool.AddTask(t2)
	pool.AddTask(t3)

	tests := []struct {
		name      string
		prefix    string
		wantID    string
		wantErr   error
		wantAnyID bool // when we just want to check an error, not a specific ID
	}{
		{
			name:   "exact full ID match",
			prefix: "abc12345-0000-0000-0000-000000000000",
			wantID: "abc12345-0000-0000-0000-000000000000",
		},
		{
			name:   "unique prefix match",
			prefix: "def",
			wantID: "def67890-0000-0000-0000-000000000000",
		},
		{
			name:    "ambiguous prefix",
			prefix:  "abc",
			wantErr: ErrAmbiguousPrefix,
		},
		{
			name:    "no match",
			prefix:  "zzz",
			wantErr: ErrTaskNotFound,
		},
		{
			name:    "empty prefix",
			prefix:  "",
			wantErr: ErrTaskNotFound,
		},
		{
			name:   "longer unique prefix",
			prefix: "abc12345-0000",
			wantID: "abc12345-0000-0000-0000-000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := pool.FindByPrefix(tt.prefix)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("FindByPrefix(%q) = nil error, want %v", tt.prefix, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("FindByPrefix(%q) error = %v, want %v", tt.prefix, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("FindByPrefix(%q) unexpected error: %v", tt.prefix, err)
			}
			if got.ID != tt.wantID {
				t.Errorf("FindByPrefix(%q).ID = %q, want %q", tt.prefix, got.ID, tt.wantID)
			}
		})
	}
}

func TestTaskPool_FindByPrefix_EmptyPool(t *testing.T) {
	t.Parallel()

	pool := NewTaskPool()
	_, err := pool.FindByPrefix("abc")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("FindByPrefix on empty pool: got %v, want ErrTaskNotFound", err)
	}
}

func TestTaskPool_FindByPrefix_SingleTask(t *testing.T) {
	t.Parallel()

	pool := NewTaskPool()
	task := &Task{ID: "unique-id-12345", Text: "Only task", Status: StatusTodo}
	pool.AddTask(task)

	got, err := pool.FindByPrefix("u")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("got ID %q, want %q", got.ID, task.ID)
	}
}
