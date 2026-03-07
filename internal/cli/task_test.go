package cli

import (
	"testing"
)

func TestShortID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		want string
	}{
		{
			name: "UUID truncated to 8",
			id:   "abc12345-6789-0123-4567-890123456789",
			want: "abc12345",
		},
		{
			name: "exactly 8 characters",
			id:   "abcdefgh",
			want: "abcdefgh",
		},
		{
			name: "shorter than 8",
			id:   "abc",
			want: "abc",
		},
		{
			name: "empty string",
			id:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shortID(tt.id)
			if got != tt.want {
				t.Errorf("shortID(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestNewTaskCmd_SubcommandStructure(t *testing.T) {
	t.Parallel()

	cmd := NewTaskCmd()
	if cmd.Use != "task" {
		t.Errorf("Use = %q, want %q", cmd.Use, "task")
	}

	subcommands := map[string]bool{}
	for _, sub := range cmd.Commands() {
		subcommands[sub.Name()] = true
	}

	for _, want := range []string{"list", "show"} {
		if !subcommands[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestTaskListCmd_FilterFlags(t *testing.T) {
	t.Parallel()

	cmd := NewTaskCmd()
	listCmd, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("finding list command: %v", err)
	}

	flagTests := []struct {
		name string
		flag string
	}{
		{"status flag", "status"},
		{"type flag", "type"},
		{"effort flag", "effort"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := listCmd.Flags().Lookup(tt.flag)
			if f == nil {
				t.Errorf("missing flag --%s", tt.flag)
			}
		})
	}
}

func TestTaskShowCmd_RequiresArg(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	root.SetArgs([]string{"task", "show"})

	err := root.Execute()
	if err == nil {
		t.Error("task show without args should fail")
	}
}

func TestTaskCmd_RegisteredInRoot(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	found := false
	for _, cmd := range root.Commands() {
		if cmd.Name() == "task" {
			found = true
			break
		}
	}
	if !found {
		t.Error("task command not registered in root")
	}
}
