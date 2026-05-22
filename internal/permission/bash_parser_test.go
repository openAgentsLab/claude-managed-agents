package permission

import "testing"

func TestParsePipeline(t *testing.T) {
	tests := []struct {
		cmd       string
		wantCmds  []string
		dangerous bool
	}{
		{cmd: "git status", wantCmds: []string{"git status"}},
		{cmd: "git log | head -20", wantCmds: []string{"git log", "head -20"}},
		{cmd: "go build && go test ./...", wantCmds: []string{"go build", "go test ./..."}},
		{cmd: "echo foo; echo bar", wantCmds: []string{"echo foo", "echo bar"}},
		{cmd: "git log || true", wantCmds: []string{"git log", "true"}},
		{cmd: `echo "a | b"`, wantCmds: []string{`echo "a | b"`}}, // pipe inside quotes not split
		{cmd: `echo 'a && b'`, wantCmds: []string{`echo 'a && b'`}},
		{cmd: "echo $(pwd)", dangerous: true}, // no subcmds parsed for dangerous
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			result := Parse(tt.cmd)
			if result.Dangerous != tt.dangerous {
				t.Errorf("Dangerous: got %v, want %v (note: %s)", result.Dangerous, tt.dangerous, result.DangerNote)
			}
			if tt.dangerous {
				return
			}
			if len(result.SubCmds) != len(tt.wantCmds) {
				t.Fatalf("SubCmds count: got %d, want %d (%v)", len(result.SubCmds), len(tt.wantCmds), result.SubCmds)
			}
			for i, sub := range result.SubCmds {
				if sub.Raw != tt.wantCmds[i] {
					t.Errorf("SubCmds[%d]: got %q, want %q", i, sub.Raw, tt.wantCmds[i])
				}
			}
		})
	}
}

func TestParseMainToken(t *testing.T) {
	tests := []struct {
		cmd  string
		main string
	}{
		{"git status", "git"},
		{"npm run build", "npm"},
		{"go test ./...", "go"},
		{"GOOS=linux go build", "go"},  // safe env var stripped
		{"timeout 30s git fetch", "git"}, // safe wrapper stripped
		{"nohup make build", "make"},
		{"time go test ./...", "go"},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			result := Parse(tt.cmd)
			if result.Dangerous {
				t.Fatalf("unexpected dangerous: %s", result.DangerNote)
			}
			if len(result.SubCmds) == 0 {
				t.Fatal("no subcmds parsed")
			}
			if result.SubCmds[0].MainToken != tt.main {
				t.Errorf("MainToken: got %q, want %q", result.SubCmds[0].MainToken, tt.main)
			}
		})
	}
}

func TestStripSafeWrappers(t *testing.T) {
	tests := []struct{ input, want string }{
		{"timeout 30s git fetch", "git fetch"},
		{"timeout --kill-after=5s 60s npm install", "npm install"},
		{"time go test ./...", "go test ./..."},
		{"nohup make build", "make build"},
		{"GOOS=linux go build", "go build"},
		{"git status", "git status"}, // nothing to strip
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := StripSafeWrappers(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchBashPattern(t *testing.T) {
	tests := []struct {
		pattern, cmd string
		want         bool
	}{
		// empty pattern matches everything
		{"", "anything", true},
		// exact match (case-insensitive)
		{"git status", "git status", true},
		{"Git Status", "git status", true},
		{"git status", "git log", false},
		// prefix :* pattern
		{"git:*", "git status", true},
		{"git:*", "git log --oneline", true},
		{"git:*", "git", true}, // exact match of prefix itself
		{"git:*", "gita status", false},
		{"npm:*", "npm run build", true},
		{"npm:*", "go build", false},
		// wildcard *
		{"npm run *", "npm run build", true},
		{"npm run *", "npm run test -- -v", true},
		{"npm run *", "npm install", false},
		{"go test *", "go test ./...", true},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+"/"+tt.cmd, func(t *testing.T) {
			got := matchBashPattern(tt.pattern, tt.cmd)
			if got != tt.want {
				t.Errorf("matchBashPattern(%q, %q) = %v, want %v", tt.pattern, tt.cmd, got, tt.want)
			}
		})
	}
}
