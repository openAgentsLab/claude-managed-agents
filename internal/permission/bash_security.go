package permission

import (
	"path/filepath"
	"regexp"
	"strings"
)

var redirectTargetRe = regexp.MustCompile(`(?:(?:\d+|&)?>>?|&>)\s*([^\s;|&]+)`)
var heredocTargetRe = regexp.MustCompile(`<<[^\n]*?>+\s*([^\s;|&]+)`)

var sensitiveSuffixes = []string{
	".bashrc", ".zshrc", ".profile", ".bash_profile", ".bash_login",
	".zprofile", ".zlogin", ".zlogout", ".tcshrc", ".cshrc",
	".inputrc", ".netrc", ".npmrc", ".pypirc",
	".gitconfig", ".gitattributes", ".gitignore_global",
}

var sensitiveDirPrefixes = []string{
	".git/", ".agent/", ".forge/", ".ssh/", ".gnupg/",
}

// CheckDangerousRedirect inspects rawCommand for output redirections that
// target sensitive files (shell configs, SSH keys, agent/forge config, etc.).
func CheckDangerousRedirect(rawCommand string) (dangerous bool, note string) {
	for _, m := range redirectTargetRe.FindAllStringSubmatch(rawCommand, -1) {
		if len(m) >= 2 && isSensitiveRedirectTarget(m[1]) {
			return true, "redirect to sensitive path: " + m[1]
		}
	}
	for _, m := range heredocTargetRe.FindAllStringSubmatch(rawCommand, -1) {
		if len(m) >= 2 && isSensitiveRedirectTarget(m[1]) {
			return true, "here-doc redirect to sensitive path: " + m[1]
		}
	}
	return false, ""
}

func isSensitiveRedirectTarget(target string) bool {
	lower := strings.ToLower(filepath.ToSlash(target))

	for _, suf := range sensitiveSuffixes {
		if strings.HasSuffix(lower, suf) {
			return true
		}
	}
	for _, pfx := range sensitiveDirPrefixes {
		if strings.Contains(lower, pfx) {
			return true
		}
	}
	if strings.HasPrefix(lower, "/etc/") || strings.HasPrefix(lower, "/usr/local/etc/") {
		return true
	}
	return false
}

type dangerousPattern struct {
	re  *regexp.Regexp
	msg string
}

var commandSubstitutionPatterns = []dangerousPattern{
	{regexp.MustCompile(`<\(`), "process substitution <()"},
	{regexp.MustCompile(`>\(`), "process substitution >()"},
	{regexp.MustCompile(`=\(`), "Zsh process substitution =()"},
	{regexp.MustCompile(`(?:^|[\s;&|])=[a-zA-Z_]`), "Zsh equals expansion (=cmd)"},
	{regexp.MustCompile(`\$\(`), "$() command substitution"},
	{regexp.MustCompile(`\$\{`), "${} parameter substitution"},
	{regexp.MustCompile(`\$\[`), "$[] legacy arithmetic expansion"},
	{regexp.MustCompile(`~\[`), "Zsh-style parameter expansion"},
	{regexp.MustCompile(`\(e:`), "Zsh-style glob qualifiers"},
	{regexp.MustCompile(`\(\+`), "Zsh glob qualifier with command execution"},
	{regexp.MustCompile(`\}\s*always\s*\{`), "Zsh always block"},
	{regexp.MustCompile(`<#`), "PowerShell comment syntax"},
	{regexp.MustCompile("(?:^|[^\\\\])`"), "backtick command substitution"},
}

var zshDangerousCommands = map[string]string{
	"zmodload": "zsh module loading gateway (zmodload)",
	"emulate":  "Zsh emulate (eval-equivalent)",
	"sysopen":  "zsh/system sysopen builtin",
	"sysread":  "zsh/system sysread builtin",
	"syswrite": "zsh/system syswrite builtin",
	"sysseek":  "zsh/system sysseek builtin",
	"zpty":     "zsh/zpty pseudo-terminal execution",
	"ztcp":     "zsh/net/tcp TCP exfiltration",
	"zsocket":  "zsh/net/socket Unix/TCP socket",
	"zf_rm":    "zsh/files builtin rm",
	"zf_mv":    "zsh/files builtin mv",
	"zf_ln":    "zsh/files builtin ln",
	"zf_chmod": "zsh/files builtin chmod",
	"zf_chown": "zsh/files builtin chown",
	"zf_mkdir": "zsh/files builtin mkdir",
	"zf_rmdir": "zsh/files builtin rmdir",
	"zf_chgrp": "zsh/files builtin chgrp",
}

// CheckDangerous inspects rawCommand for shell constructs that can execute
// arbitrary code in a way that defeats static permission matching.
func CheckDangerous(rawCommand string) (dangerous bool, note string) {
	for _, p := range commandSubstitutionPatterns {
		if p.re.MatchString(rawCommand) {
			return true, p.msg
		}
	}
	if dangerous, note = CheckDangerousRedirect(rawCommand); dangerous {
		return true, note
	}
	return false, ""
}

// CheckDangerousToken checks whether a single command token is a known zsh
// dangerous builtin.
func CheckDangerousToken(token string) (dangerous bool, note string) {
	if msg, ok := zshDangerousCommands[token]; ok {
		return true, msg
	}
	return false, ""
}
