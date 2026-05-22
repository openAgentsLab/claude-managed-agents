---
name: commit
description: Review staged changes and create a conventional git commit
when_to_use: When you want to commit staged changes following project conventions
argument-hint: "optional: context or commit message hint"
context: fork
allowed-tools:
  - bash
  - git_status
  - git_diff
  - git_add
  - git_commit
---

Review the currently staged changes and create a well-formed conventional commit.

Steps:
1. Run `git status` and `git diff --cached` to understand what is staged.
2. Write a concise commit message following the Conventional Commits format:
   `<type>(<scope>): <description>`
   where type is one of: feat, fix, docs, style, refactor, test, chore.
3. Keep the subject line under 72 characters.
4. Run `git commit -m "<message>"`.

$ARGUMENTS
