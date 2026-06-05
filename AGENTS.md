# Fusion Git Desk - Codex Notes

## Project

Fusion Git Desk is a Wails v2 desktop app for managing many Git repositories from one workspace.

- Backend: Go, package `main`
- Desktop shell: Wails v2
- Frontend: Vue 3 + TypeScript + Vite
- Git integration: system `git` executable through `os/exec`

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
