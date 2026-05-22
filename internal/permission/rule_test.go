package permission

import "testing"

func TestParseRuleString(t *testing.T) {
	tests := []struct {
		input    string
		behavior Behavior
		wantTool string
		wantPat  string
		wantErr  bool
	}{
		{input: "bash", behavior: BehaviorAllow, wantTool: "bash"},
		{input: "read_file", behavior: BehaviorDeny, wantTool: "read_file"},
		{input: "Bash(git:*)", behavior: BehaviorAllow, wantTool: "Bash", wantPat: "git:*"},
		{input: "WebFetch(domain:github.com)", behavior: BehaviorAllow, wantTool: "WebFetch", wantPat: "domain:github.com"},
		{input: "write_file(path:/tmp)", behavior: BehaviorDeny, wantTool: "write_file", wantPat: "path:/tmp"},
		{input: "  bash  ", behavior: BehaviorAllow, wantTool: "bash"}, // trim whitespace
		{input: "", wantErr: true},
		{input: "bash(unclosed", wantErr: true},
		{input: "(no-tool)", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			r, err := ParseRuleString(tt.input, tt.behavior, SourceProject)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.ToolName != tt.wantTool {
				t.Errorf("ToolName: got %q, want %q", r.ToolName, tt.wantTool)
			}
			if r.Pattern != tt.wantPat {
				t.Errorf("Pattern: got %q, want %q", r.Pattern, tt.wantPat)
			}
			if r.Behavior != tt.behavior {
				t.Errorf("Behavior: got %q, want %q", r.Behavior, tt.behavior)
			}
		})
	}
}

func TestRuleString(t *testing.T) {
	tests := []struct {
		rule Rule
		want string
	}{
		{Rule{ToolName: "bash", Pattern: "git:*", Behavior: BehaviorAllow}, "bash(git:*)"},
		{Rule{ToolName: "read_file", Behavior: BehaviorAllow}, "read_file"},
		{Rule{ToolName: "", Pattern: "", Behavior: BehaviorDeny}, "*"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.rule.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
