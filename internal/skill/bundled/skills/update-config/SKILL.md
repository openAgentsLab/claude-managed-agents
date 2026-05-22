---
name: update-config
description: Configure the agent via settings.json — permissions, hooks, env vars, and tool rules
when_to_use: Use when the user wants to configure automated behaviours ("from now on when X", "each time X"), grant or revoke tool permissions ("allow bash", "deny write to /etc"), set environment variables, or modify .forge/settings.json and ~/.forge/settings.json. For simple preferences (model choice, verbosity) use the Config tool instead.
argument-hint: "what to configure, e.g. 'allow npm commands' or 'run go test before committing'"
context: inline
allowed-tools:
  - read_file
---

Help the user configure the agent by editing the appropriate settings file.

$ARGUMENTS

## Settings file locations

| Scope | Path | Who sees it |
|-------|------|-------------|
| User (global) | `~/.forge/settings.json` | You, all projects |
| Project | `.forge/settings.json` | Everyone in the repo (commit this) |
| Local | `.forge/settings.local.json` | You only, this project (gitignore this) |

**Priority**: local → project → user (local wins on conflict).

## Settings schema

```jsonc
{
  // Tool permission rules — evaluated top-to-bottom, first match wins.
  "permissions": {
    "allow": [
      "bash(npm *)",
      "bash(go test *)",
      "write_file(src/**)"
    ],
    "deny": [
      "bash(rm -rf *)",
      "write_file(/etc/**)"
    ]
  },

  // Environment variables injected into every tool execution.
  "env": {
    "GOFLAGS": "-mod=vendor"
  },

  // Hooks — shell commands the harness runs on specific events.
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "bash",
        "hooks": [{ "command": "echo 'about to run bash'" }]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "write_file",
        "hooks": [{ "command": "gofmt -w $FORGE_FILE_PATH 2>/dev/null || true" }]
      }
    ],
    "Stop": [
      {
        "hooks": [{ "command": "notify-send 'Agent done'" }]
      }
    ]
  }
}
```

## Steps

1. Read the current contents of the relevant settings file (user or project scope based on the request).
2. Determine the minimal change needed.
3. Show the user the proposed diff before writing.
4. Write the updated JSON, preserving all existing settings.
5. Confirm what was changed and what it does.
