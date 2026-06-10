package main

import (
	"context"
	"errors"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx      context.Context
	git      *GitService
	settings *SettingsStore
}

type InitialState struct {
	Settings Settings `json:"settings"`
	HasGit   bool     `json:"hasGit"`
}

func NewApp() *App {
	return &App{
		git:      NewGitService(),
		settings: NewSettingsStore("fusion-git-desk", "settings.json"),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetInitialState() (InitialState, error) {
	settings, err := a.settings.Load()
	if err != nil {
		return InitialState{}, err
	}
	return InitialState{Settings: settings, HasGit: a.git.HasGit()}, nil
}

func (a *App) SaveSettings(settings Settings) error {
	return a.settings.Save(settings.Normalize())
}

func (a *App) PickDirectory() (string, error) {
	if a.ctx == nil {
		return "", errors.New("app context is not ready")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Select a workspace folder"})
}

func (a *App) ScanRepositories(root string, maxDepth int) (ScanResponse, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return ScanResponse{}, errors.New("root path is required")
	}
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	if maxDepth > MaxScanDepth {
		maxDepth = MaxScanDepth
	}

	repositories, err := a.git.Scan(root, maxDepth)
	response := ScanResponse{
		Root:         root,
		MaxDepth:     maxDepth,
		Repositories: repositories,
		ScannedAt:    nowISO(),
	}
	if err != nil {
		response.Error = err.Error()
	}

	settings, loadErr := a.settings.Load()
	if loadErr == nil {
		settings.LastRoot = root
		settings.MaxDepth = maxDepth
		_ = a.settings.Save(settings.Normalize())
	}
	return response, nil
}

func (a *App) RefreshRepository(path string) (Repository, error) {
	return a.git.Inspect(path)
}

func (a *App) GetRepositoryDiff(path string, mode string) (DiffResponse, error) {
	return a.git.Diff(path, mode)
}

func (a *App) GetRepositoryFileDiff(path string, mode string, filePath string) (DiffResponse, error) {
	return a.git.FileDiff(path, mode, filePath)
}

func (a *App) GetBranches(path string) (BranchResponse, error) {
	return a.git.Branches(path)
}

func (a *App) CheckoutBranch(path string, branch string) (CommandResult, error) {
	return a.git.CheckoutBranch(path, branch)
}

func (a *App) CommitRepository(path string, message string) (CommandResult, error) {
	return a.git.CommitRepository(path, message)
}

func (a *App) StageFile(path string, filePath string) (CommandResult, error) {
	return a.git.StageFile(path, filePath)
}

func (a *App) UnstageFile(path string, filePath string) (CommandResult, error) {
	return a.git.UnstageFile(path, filePath)
}

func (a *App) UpdateRepositories(request UpdateRequest) ([]UpdateResult, error) {
	return a.git.UpdateRepositories(request), nil
}
