# Fusion Git Desk - Codex Notes

## Project

Fusion Git Desk is a Wails v2 desktop app for managing many Git repositories from one workspace.

- Backend: Go, package `main`
- Desktop shell: Wails v2
- Frontend: Vue 3 + TypeScript + Vite
- Git integration: system `git` executable through `os/exec`

## Product Direction

Fusion Git Desk is a lightweight multi-repository Git operations desk. Its core job is to help users quickly answer:

- What changed across all repositories in this workspace?
- Which repositories are behind, ahead, conflicted, or missing upstream?
- Which repositories can be safely fetched or fast-forward pulled?
- What changed in the selected repository, and can the user safely stage and commit a small change?

It is not trying to replace an IDE Git panel. Do not prioritize deep or speculative features before the basic multi-repository status, diff, safe sync, and small commit loop is reliable.

## Evolution Loop

Before choosing an implementation target, check the foundation first:

1. Can the app scan a workspace and show repository state clearly?
2. Can the user inspect changed files and useful diffs without UI stalls?
3. Are fetch and pull explicit, bounded, and safe for dirty repositories?
4. Are stage, unstage, and commit limited, understandable, and protected by tests?
5. Are errors visible enough for the user to know what happened?
6. Do the local build, Go tests, and release workflow still form a believable verification path?

Only optimize problems that are user-visible, safety-related, release-blocking, or backed by a concrete measurement. Avoid broad refactors, speculative architecture, or advanced feature work when the foundation loop has an open P0/P1 gap.

## Commands

```bash
cd frontend
pnpm install
pnpm build

go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
wails dev
wails build
```

## Conventions

- Keep Git mutations explicit. Automatic pull should stay limited to clean repositories unless the user opts out.
- Prefer `git status --porcelain` and structured command output over parsing human-oriented text.
- Keep long-running Git commands bounded by timeouts.
- On Windows, Git subprocesses must use hidden-window startup to avoid spawning visible Terminal windows.
