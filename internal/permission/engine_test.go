package permission

import "testing"

// --- Mode behavior ---

func TestPlanModeReadOnlyAllowed(t *testing.T) {
	eng := NewEngine(ModePlan)
	d := eng.Check("read_file", `{"file_path":"/foo"}`, true)
	if d.Behavior != BehaviorAllow {
		t.Errorf("plan mode: read-only tool should be allowed, got %s", d.Behavior)
	}
}

func TestPlanModeWriteDenied(t *testing.T) {
	eng := NewEngine(ModePlan)
	d := eng.Check("write_file", `{"file_path":"/foo"}`, false)
	if d.Behavior != BehaviorDeny {
		t.Errorf("plan mode: write tool should be denied, got %s", d.Behavior)
	}
}

func TestDefaultModeAllowsWithoutRules(t *testing.T) {
	eng := NewEngine(ModeDefault)
	d := eng.Check("read_file", `{}`, false)
	if d.Behavior != BehaviorAllow {
		t.Errorf("default mode: no rules should allow, got %s", d.Behavior)
	}
}

// --- Rule precedence ---

func TestAllowRuleGrantsAccess(t *testing.T) {
	eng := NewEngine(ModeDefault)
	eng.AddRule(Rule{ToolName: "read_file", Behavior: BehaviorAllow, Source: SourceProject})
	d := eng.Check("read_file", `{}`, false)
	if d.Behavior != BehaviorAllow {
		t.Errorf("expected Allow from rule, got %s", d.Behavior)
	}
}

func TestHigherSourceDenyOverridesLowerAllow(t *testing.T) {
	eng := NewEngine(ModeDefault)
	eng.AddRule(Rule{ToolName: "bash", Pattern: "git:*", Behavior: BehaviorAllow, Source: SourceUser})
	eng.AddRule(Rule{ToolName: "bash", Pattern: "git:*", Behavior: BehaviorDeny, Source: SourceProject})
	d := eng.Check("bash", `{"command":"git status"}`, false)
	if d.Behavior != BehaviorDeny {
		t.Errorf("higher-source deny should win, got %s", d.Behavior)
	}
}

func TestNoMatchingRuleAllows(t *testing.T) {
	eng := NewEngine(ModeDefault)
	d := eng.Check("read_file", `{"file_path":"/foo"}`, false)
	if d.Behavior != BehaviorAllow {
		t.Errorf("no matching rule should allow, got %s", d.Behavior)
	}
}

// --- Bash-specific checks ---

func TestBashDangerousCommandDenied(t *testing.T) {
	eng := NewEngine(ModeDefault)
	d := eng.Check("bash", `{"command":"echo $(whoami)"}`, false)
	if d.Behavior != BehaviorDeny {
		t.Errorf("dangerous bash should be denied, got %s", d.Behavior)
	}
	if d.ReasonType != "safetyCheck" {
		t.Errorf("reason should be safetyCheck, got %q", d.ReasonType)
	}
}

func TestBashPrefixRuleAllows(t *testing.T) {
	eng := NewEngine(ModeDefault)
	eng.AddRule(Rule{ToolName: "bash", Pattern: "git:*", Behavior: BehaviorAllow, Source: SourceProject})

	d := eng.Check("bash", `{"command":"git status"}`, false)
	if d.Behavior != BehaviorAllow {
		t.Errorf("git:* should allow 'git status', got %s", d.Behavior)
	}

	d = eng.Check("bash", `{"command":"git log --oneline -10"}`, false)
	if d.Behavior != BehaviorAllow {
		t.Errorf("git:* should allow 'git log', got %s", d.Behavior)
	}

	d = eng.Check("bash", `{"command":"npm install"}`, false)
	if d.Behavior != BehaviorAllow {
		t.Errorf("git:* should not deny 'npm install', got %s", d.Behavior)
	}
}

func TestBashPipelineUnmatchedPartAllows(t *testing.T) {
	eng := NewEngine(ModeDefault)
	eng.AddRule(Rule{ToolName: "bash", Pattern: "git:*", Behavior: BehaviorAllow, Source: SourceProject})

	d := eng.Check("bash", `{"command":"git log | head -20"}`, false)
	if d.Behavior != BehaviorAllow {
		t.Errorf("pipeline with unmatched part should allow in default mode, got %s", d.Behavior)
	}
}

func TestBashDenyRuleBlocksInPipeline(t *testing.T) {
	eng := NewEngine(ModeDefault)
	eng.AddRule(Rule{ToolName: "bash", Pattern: "rm:*", Behavior: BehaviorDeny, Source: SourceProject})
	eng.AddRule(Rule{ToolName: "bash", Pattern: "git:*", Behavior: BehaviorAllow, Source: SourceProject})

	d := eng.Check("bash", `{"command":"git log | rm -rf /tmp"}`, false)
	if d.Behavior != BehaviorDeny {
		t.Errorf("deny rule in pipeline should deny overall, got %s", d.Behavior)
	}
}

// --- Protected path checks ---

func TestWriteFileProtectedPathDenied(t *testing.T) {
	eng := NewEngine(ModeDefault)
	cases := []string{
		"/project/.forge/settings.json",
		"/home/user/.bashrc",
		"/repo/.git/hooks/pre-commit",
		"/home/user/.ssh/authorized_keys",
	}
	for _, path := range cases {
		d := eng.Check("write_file", `{"file_path":"`+path+`"}`, false)
		if d.Behavior != BehaviorDeny {
			t.Errorf("write to %q should be denied, got %s", path, d.Behavior)
		}
	}
}

// --- WithMode ---

func TestWithModePreservesRules(t *testing.T) {
	eng := NewEngine(ModeDefault)
	eng.AddRule(Rule{ToolName: "bash", Pattern: "rm:*", Behavior: BehaviorDeny, Source: SourceProject})

	plan := eng.WithMode(ModePlan)
	if plan.Mode() != ModePlan {
		t.Errorf("WithMode should set mode to plan, got %s", plan.Mode())
	}
	// deny rule should still fire in plan mode (belt-and-suspenders)
	d := plan.Check("bash", `{"command":"rm -rf /tmp"}`, false)
	if d.Behavior != BehaviorDeny {
		t.Errorf("deny rule should still fire after WithMode, got %s", d.Behavior)
	}
	// original engine unchanged
	if eng.Mode() != ModeDefault {
		t.Errorf("original engine mode should be unchanged, got %s", eng.Mode())
	}
}
