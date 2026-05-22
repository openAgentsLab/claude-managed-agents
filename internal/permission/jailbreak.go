package permission

import (
	"fmt"
	"regexp"
)

// jailbreakPattern pairs a compiled regex with a human-readable description of
// the attack class it detects. All patterns are case-insensitive.
type jailbreakPattern struct {
	re  *regexp.Regexp
	msg string
}

// jailbreakPatterns is the ordered list of known prompt-injection / jailbreak
// signatures. Add new patterns here; order is irrelevant (all are tested).
//
// Design notes:
//   - Patterns are anchored to word boundaries where possible to reduce false positives.
//   - Legitimate messages that match these patterns are extremely rare in a coding
//     assistant context; the Haiku classifier is a second line of defence for edge cases.
var jailbreakPatterns = []jailbreakPattern{
	// Classic instruction override attempts
	{regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|prior|above|earlier|your)\s+instructions?`), "instruction override: ignore previous instructions"},
	{regexp.MustCompile(`(?i)disregard\s+(all\s+)?(previous|prior|above|earlier|your)\s+instructions?`), "instruction override: disregard previous instructions"},
	{regexp.MustCompile(`(?i)forget\s+(all\s+)?(previous|prior|above|earlier|your)\s+instructions?`), "instruction override: forget previous instructions"},
	{regexp.MustCompile(`(?i)override\s+(your\s+)?(instructions?|constraints?|guidelines?|rules?|programming)`), "instruction override attempt"},

	// Persona hijacking
	{regexp.MustCompile(`(?i)\byou\s+are\s+now\s+(a\s+)?(DAN|jailbreak|unrestricted|different\s+AI|an?\s+AI\s+without)`), "persona hijack: you are now"},
	{regexp.MustCompile(`(?i)\bpretend\s+(you\s+)?(have\s+no|don.t\s+have|are\s+without)\s+(restrictions?|guidelines?|ethics?|rules?|constraints?|limits?)`), "persona hijack: pretend no restrictions"},
	{regexp.MustCompile(`(?i)\bact\s+as\s+if\s+you\s+(have\s+no|don.t\s+have|are\s+without)\s+(restrictions?|guidelines?|ethics?|rules?|constraints?)`), "persona hijack: act as if unrestricted"},
	{regexp.MustCompile(`(?i)\bact\s+as\s+(DAN|jailbreak|an?\s+unrestricted)\b`), "persona hijack: act as DAN"},

	// System prompt extraction
	{regexp.MustCompile(`(?i)(print|output|repeat|reveal|show|display|tell\s+me|return)\s+(your\s+)?(full\s+|complete\s+|entire\s+|raw\s+)?(system\s+prompt|instructions?|prompt|configuration)`), "system prompt extraction attempt"},
	{regexp.MustCompile(`(?i)what\s+(are|were)\s+your\s+(exact\s+)?(original\s+)?(instructions?|system\s+prompt|guidelines?|rules?)`), "system prompt inquiry"},

	// Synthetic authority injection
	{regexp.MustCompile(`(?i)\[SYSTEM\s*(?:OVERRIDE|UPDATE|MESSAGE|PROMPT|INSTRUCTION)\]`), "synthetic system tag injection"},
	{regexp.MustCompile(`(?i)<SYSTEM\s*(?:OVERRIDE|UPDATE|MESSAGE|PROMPT|INSTRUCTION)>`), "synthetic XML system tag"},
	{regexp.MustCompile(`(?i)###+\s*SYSTEM\s*(OVERRIDE|UPDATE|MESSAGE|PROMPT)`), "synthetic markdown system header"},

	// Token boundary / encoding attacks
	{regexp.MustCompile(`(?i)base64\s*decode\s*(and\s+execute|then\s+run|the\s+following)`), "base64 execution attempt"},
	{regexp.MustCompile(`(?i)translate\s+(this|the\s+following)\s+(from\s+base64|from\s+hex|into\s+python|into\s+bash)\s+and\s+(run|execute)`), "encoded execution attempt"},

	// Role-play escape
	{regexp.MustCompile(`(?i)\bstop\s+being\s+(an?\s+AI|an?\s+assistant|claude|forge)\b`), "role-play escape: stop being AI"},
	{regexp.MustCompile(`(?i)\byour\s+true\s+(self|form|nature|identity)\s+(has|is|would)\s+(no|not|never|without)\s+(restrictions?|guidelines?|ethics?)`), "true self jailbreak"},
}

// ScanInput checks userText for known jailbreak / prompt-injection patterns.
// Returns (true, "") when the input is clean, or (false, reason) when a pattern
// fires. This is a fast, code-only check; the Haiku SafetyClassifier is the
// second line of defence for novel attacks not covered by these patterns.
func ScanInput(userText string) (safe bool, reason string) {
	for _, p := range jailbreakPatterns {
		if p.re.MatchString(userText) {
			return false, fmt.Sprintf("jailbreak pattern detected: %s", p.msg)
		}
	}
	return true, ""
}
