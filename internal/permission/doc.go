// Package permission implements a multi-layer permission system for tool invocations.
//
// Rules are loaded from settings files at startup (user-level and project-level),
// supplemented by CLI arguments, and extended at runtime when the user approves a
// request and chooses "always allow".
//
// The decision pipeline for a Bash tool call:
//
//  1. If Mode is Plan and the tool is read-only → allow immediately.
//  2. If Mode is Plan and the tool is writable → deny immediately.
//  3. If Mode is BypassPermissions → allow immediately.
//  4. Run [CheckDangerous] on the raw command string; any hit → deny.
//  5. Strip safe wrapper commands (timeout, nice, …) and safe env-var prefixes.
//  6. Split the pipeline into sub-commands.
//  7. For each sub-command, walk rules: deny first, then allow, then ask.
//  8. Aggregate sub-command results (any deny → deny; any ask → ask).
//
// For non-Bash tools the pipeline is simpler: skip steps 4-6 and match the
// raw argsJSON against tool-level rules.
package permission
