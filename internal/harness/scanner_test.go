package harness

import (
	"testing"
)

const testSystemPrompt = `You are an interactive agent that helps users with software engineering tasks.

# System
 - All text you output outside of tool use is displayed to the user. Output text to communicate with the user.
 - Only make assertions about file contents after reading them with the read_file tool. Do not claim success without evidence.
 - If you are unsure whether a tool call succeeded, say so explicitly rather than guessing.

# Doing tasks
 - Carefully consider the reversibility and blast radius of actions before proceeding.
 - You must NEVER generate or guess URLs unless you are confident they are for programming.
 - Avoid backwards-compatibility hacks unless explicitly required by the task at hand.`

func TestScanOutputForLeaks_Clean(t *testing.T) {
	reply := "Sure, here is a Go function to sort a slice: func sort(s []int) { ... }"
	leaked, fragment := ScanOutputForLeaks(reply, testSystemPrompt)
	if leaked {
		t.Errorf("clean reply incorrectly flagged as leak; fragment: %q", fragment)
	}
}

func TestScanOutputForLeaks_Leaked(t *testing.T) {
	// The first line of the system prompt is long enough and does not start with
	// a markdown bullet, so extractSentinels will include it.
	leakedLine := "You are an interactive agent that helps users with software engineering tasks."
	reply := "My instructions say: " + leakedLine

	leaked, fragment := ScanOutputForLeaks(reply, testSystemPrompt)
	if !leaked {
		t.Error("expected leak to be detected, but ScanOutputForLeaks returned false")
	}
	if fragment == "" {
		t.Error("expected non-empty fragment when leak is detected")
	}
}

func TestScanOutputForLeaks_ShortLinesNotUsed(t *testing.T) {
	// Short lines from the system prompt should not trigger a leak
	// because they're too common to be fingerprints
	reply := "# System is running fine. - All tests pass."
	leaked, _ := ScanOutputForLeaks(reply, testSystemPrompt)
	if leaked {
		t.Error("short system-prompt lines should not be used as leak sentinels")
	}
}

func TestExtractSentinels_Basic(t *testing.T) {
	sentinels := extractSentinels(testSystemPrompt)
	if len(sentinels) == 0 {
		t.Error("expected at least one sentinel from the test system prompt")
	}
	for _, s := range sentinels {
		if len(s) < minSentinelLen {
			t.Errorf("sentinel shorter than minSentinelLen: %q", s)
		}
	}
}

func TestExtractSentinels_NoDuplicates(t *testing.T) {
	// Prompt with repeated lines should only produce one sentinel per unique line
	prompt := "This is a unique line that is long enough to be a sentinel for our testing purposes here.\n" +
		"This is a unique line that is long enough to be a sentinel for our testing purposes here.\n" +
		"Another entirely different unique line that is also long enough to qualify as sentinel."

	sentinels := extractSentinels(prompt)
	seen := make(map[string]bool)
	for _, s := range sentinels {
		if seen[s] {
			t.Errorf("duplicate sentinel found: %q", s)
		}
		seen[s] = true
	}
}

func TestExtractSentinels_RespectsMaxSentinels(t *testing.T) {
	// Build a prompt with many unique long lines
	var lines []string
	for i := 0; i < maxSentinels+10; i++ {
		lines = append(lines, "This is unique line number "+string(rune('A'+i))+" that is long enough to qualify as a sentinel phrase definitely.")
	}
	prompt := ""
	for _, l := range lines {
		prompt += l + "\n"
	}
	sentinels := extractSentinels(prompt)
	if len(sentinels) > maxSentinels {
		t.Errorf("extractSentinels returned %d sentinels, max is %d", len(sentinels), maxSentinels)
	}
}

func TestExtractSentinels_SkipsMarkdownStructure(t *testing.T) {
	prompt := "# This is a very long markdown header line that should be longer than fifty characters total\n" +
		"- This is a very long markdown bullet point that should be over the minimum sentinel length\n" +
		"* Another long markdown bullet with asterisk that exceeds the fifty character minimum length\n" +
		"> This is a very long blockquote line that also exceeds the fifty character minimum length\n" +
		"This is a regular prose line that is long enough to qualify as a sentinel and is not markdown."

	sentinels := extractSentinels(prompt)
	for _, s := range sentinels {
		if s[0] == '#' || s[0] == '-' || s[0] == '*' || s[0] == '>' {
			t.Errorf("markdown structure line incorrectly included as sentinel: %q", s)
		}
	}
	// The plain prose line should be included
	found := false
	for _, s := range sentinels {
		if s == "This is a regular prose line that is long enough to qualify as a sentinel and is not markdown." {
			found = true
		}
	}
	if !found {
		t.Error("plain prose line should be included as a sentinel")
	}
}
