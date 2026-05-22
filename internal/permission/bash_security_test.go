package permission

import "testing"

func TestCheckDangerous(t *testing.T) {
	dangerous := []struct {
		cmd      string
		wantNote string
	}{
		{`echo $(pwd)`, "$() command substitution"},
		{"echo `whoami`", "backtick command substitution"},
		{`x=${HOME}`, "${} parameter substitution"},
		{`cmd <(other)`, "process substitution <()"},
		{`cmd >(other)`, "process substitution >()"},
	}
	for _, tt := range dangerous {
		t.Run("dangerous/"+tt.cmd, func(t *testing.T) {
			d, note := CheckDangerous(tt.cmd)
			if !d {
				t.Errorf("expected dangerous, got safe")
			}
			if tt.wantNote != "" && note != tt.wantNote {
				t.Errorf("note: got %q, want %q", note, tt.wantNote)
			}
		})
	}

	safe := []string{
		"git status",
		"ls -la",
		"go build ./...",
		"npm run build",
		"echo hello",
	}
	for _, cmd := range safe {
		t.Run("safe/"+cmd, func(t *testing.T) {
			d, _ := CheckDangerous(cmd)
			if d {
				t.Errorf("expected safe, got dangerous")
			}
		})
	}
}

func TestCheckDangerousRedirect(t *testing.T) {
	tests := []struct {
		cmd       string
		dangerous bool
	}{
		{"echo foo > .bashrc", true},
		{"echo foo >> ~/.gitconfig", true},
		{"cat > .ssh/authorized_keys", true},
		{"echo x > .forge/settings.json", true},
		{"echo foo > output.txt", false},
		{"cat file > /tmp/result", false},
		{"ls > results.log", false},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			d, _ := CheckDangerousRedirect(tt.cmd)
			if d != tt.dangerous {
				t.Errorf("got %v, want %v", d, tt.dangerous)
			}
		})
	}
}

func TestCheckDangerousToken(t *testing.T) {
	dangerous := []string{"zmodload", "emulate", "zpty", "ztcp"}
	for _, tok := range dangerous {
		t.Run("dangerous/"+tok, func(t *testing.T) {
			d, _ := CheckDangerousToken(tok)
			if !d {
				t.Errorf("expected dangerous token")
			}
		})
	}

	safe := []string{"git", "go", "npm", "ls", "echo"}
	for _, tok := range safe {
		t.Run("safe/"+tok, func(t *testing.T) {
			d, _ := CheckDangerousToken(tok)
			if d {
				t.Errorf("expected safe token")
			}
		})
	}
}
