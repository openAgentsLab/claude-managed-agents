---
name: simplify
description: Review changed code for reuse, quality, and efficiency, then fix any issues found
when_to_use: After writing or modifying code, to review and improve it. Invoke after completing a feature or fix.
argument-hint: "optional: file paths or focus area"
context: fork
allowed-tools:
  - bash
  - read_file
  - write_file
  - file_edit
  - glob
  - grep_file
  - git_diff
  - git_status
---

Review the recently changed code for opportunities to improve quality, reduce duplication, and increase efficiency. Run three focused reviews in parallel, then apply the fixes.

$ARGUMENTS

## Phase 1: Identify changed files

Run `git diff HEAD --name-only` (or inspect the files specified in $ARGUMENTS) to determine which files changed.

## Phase 2: Three parallel reviews

Launch three concurrent review passes over the changed files:

### Review A — Code Reuse
- Search the codebase for existing utilities, helpers, or patterns that the new code duplicates.
- Flag functions or logic blocks that already exist elsewhere and could be reused.
- Flag inline logic that could call an existing helper instead of reimplementing it.

### Review B — Code Quality
- Redundant state or cached values that are never invalidated correctly.
- Parameter sprawl: functions with too many arguments that should be grouped into a struct.
- Copy-paste variations: near-identical blocks that differ in one field and should be parameterised.
- Leaky abstractions: implementation details exposed through public interfaces.
- Unnecessary complexity: overly defensive checks for impossible states.
- Unclear naming: variables or functions whose name does not reflect their purpose.

### Review C — Efficiency
- Unnecessary work: computations inside loops that could be hoisted out.
- Missed concurrency: sequential operations that are independent and could run in parallel.
- Overly broad operations: reading an entire file when only a few lines are needed.
- Redundant existence checks (TOCTOU): stat-then-open patterns that should be a single atomic operation.
- Memory issues: accumulating unbounded slices or maps without a size cap.

## Phase 3: Apply fixes

For each finding from the three reviews:
1. Confirm the fix is safe (does not change observable behaviour).
2. Apply it directly using the edit tools.
3. Do NOT add docstrings, comments, or type annotations to code you did not change.
4. Do NOT refactor beyond what the finding specifically requires.

## Phase 4: Summarise

List what was changed and why, grouped by review category (Reuse / Quality / Efficiency). If nothing needed fixing in a category, say so explicitly.
