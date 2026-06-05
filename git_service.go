package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	diffModeWorking = "working"
	diffModeStaged  = "staged"
	diffModeHead    = "head"

	maxUntrackedFullDiffFiles = 30
	maxUntrackedPreviewBytes  = 180000
	maxScanInspectWorkers     = 12
	maxUpdateWorkers          = 4
)

type GitService struct {
	commandTimeout time.Duration
	diffByteLimit  int
}

type ScanResponse struct {
	Root         string       `json:"root"`
	MaxDepth     int          `json:"maxDepth"`
	Repositories []Repository `json:"repositories"`
	ScannedAt    string       `json:"scannedAt"`
	Error        string       `json:"error,omitempty"`
}

type Repository struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	Branch      string     `json:"branch"`
	Head        string     `json:"head"`
	Upstream    string     `json:"upstream"`
	RemoteURL   string     `json:"remoteUrl"`
	HasUpstream bool       `json:"hasUpstream"`
	IsClean     bool       `json:"isClean"`
	Ahead       int        `json:"ahead"`
	Behind      int        `json:"behind"`
	Status      RepoStatus `json:"status"`
	LastCommit  CommitInfo `json:"lastCommit"`
	InspectedAt string     `json:"inspectedAt"`
	Error       string     `json:"error,omitempty"`
}

type RepoStatus struct {
	Added      int           `json:"added"`
	Modified   int           `json:"modified"`
	Deleted    int           `json:"deleted"`
	Renamed    int           `json:"renamed"`
	Copied     int           `json:"copied"`
	Untracked  int           `json:"untracked"`
	Conflicted int           `json:"conflicted"`
	Staged     int           `json:"staged"`
	Unstaged   int           `json:"unstaged"`
	Files      []ChangedFile `json:"files"`
}

type ChangedFile struct {
	Path     string `json:"path"`
	OldPath  string `json:"oldPath,omitempty"`
	Status   string `json:"status"`
	Staged   bool   `json:"staged"`
	Unstaged bool   `json:"unstaged"`
}

type CommitInfo struct {
	Hash         string `json:"hash"`
	Author       string `json:"author"`
	RelativeTime string `json:"relativeTime"`
	Subject      string `json:"subject"`
}

type BranchResponse struct {
	Path      string       `json:"path"`
	Current   string       `json:"current"`
	Branches  []BranchInfo `json:"branches"`
	Generated string       `json:"generated"`
}

type BranchInfo struct {
	Name         string `json:"name"`
	Current      bool   `json:"current"`
	Remote       bool   `json:"remote"`
	Upstream     string `json:"upstream"`
	Commit       string `json:"commit"`
	RelativeTime string `json:"relativeTime"`
	Subject      string `json:"subject"`
}

type DiffResponse struct {
	Path      string     `json:"path"`
	Mode      string     `json:"mode"`
	Target    string     `json:"target,omitempty"`
	Files     []DiffFile `json:"files"`
	Raw       string     `json:"raw"`
	Truncated bool       `json:"truncated"`
	Generated string     `json:"generated"`
	Note      string     `json:"note,omitempty"`
	Error     string     `json:"error,omitempty"`
}

type DiffFile struct {
	OldPath   string     `json:"oldPath"`
	NewPath   string     `json:"newPath"`
	Status    string     `json:"status"`
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	Lines     []DiffLine `json:"lines"`
}

type DiffLine struct {
	Kind    string `json:"kind"`
	Content string `json:"content"`
	OldLine int    `json:"oldLine"`
	NewLine int    `json:"newLine"`
}

type UpdateRequest struct {
	Paths     []string `json:"paths"`
	Mode      string   `json:"mode"`
	OnlyClean bool     `json:"onlyClean"`
	Prune     bool     `json:"prune"`
}

type UpdateResult struct {
	Path       string      `json:"path"`
	Mode       string      `json:"mode"`
	Skipped    bool        `json:"skipped"`
	Success    bool        `json:"success"`
	Message    string      `json:"message"`
	Stdout     string      `json:"stdout"`
	Stderr     string      `json:"stderr"`
	Before     *Repository `json:"before,omitempty"`
	After      *Repository `json:"after,omitempty"`
	FinishedAt string      `json:"finishedAt"`
}

type CommandResult struct {
	Path       string `json:"path"`
	Command    string `json:"command"`
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	FinishedAt string `json:"finishedAt"`
}

func NewGitService() *GitService {
	return &GitService{
		commandTimeout: time.Duration(DefaultGitCommandTimeoutMillis) * time.Millisecond,
		diffByteLimit:  DefaultDiffDisplayByteLimit,
	}
}

func (g *GitService) HasGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func (g *GitService) Scan(root string, maxDepth int) ([]Repository, error) {
	if !g.HasGit() {
		return nil, errors.New("git executable was not found in PATH")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	found := make(map[string]struct{})
	repoPaths := make([]string, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.IsDir() {
			return nil
		}
		if path != root && shouldSkipDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		if directoryDepth(root, path) > maxDepth {
			return filepath.SkipDir
		}
		if !hasGitMarker(path) {
			return nil
		}
		topLevel := filepath.Clean(path)
		if _, ok := found[topLevel]; ok {
			return filepath.SkipDir
		}
		found[topLevel] = struct{}{}
		repoPaths = append(repoPaths, topLevel)
		if path == root {
			return nil
		}
		return filepath.SkipDir
	})

	repositories := g.inspectRepositories(repoPaths)
	sort.Slice(repositories, func(i, j int) bool {
		return strings.ToLower(repositories[i].Path) < strings.ToLower(repositories[j].Path)
	})
	return repositories, err
}

func (g *GitService) inspectRepositories(paths []string) []Repository {
	if len(paths) == 0 {
		return nil
	}

	type inspectResult struct {
		index int
		repo  Repository
	}

	jobs := make(chan int)
	results := make(chan inspectResult, len(paths))
	workerCount := boundedWorkerCount(len(paths), maxScanInspectWorkers)
	var wg sync.WaitGroup

	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				path := paths[index]
				repo, inspectErr := g.Inspect(path)
				if inspectErr != nil {
					repo = Repository{ID: repoID(path), Name: filepath.Base(path), Path: path, InspectedAt: nowISO(), Error: inspectErr.Error()}
				}
				results <- inspectResult{index: index, repo: repo}
			}
		}()
	}

	go func() {
		for index := range paths {
			jobs <- index
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	repositories := make([]Repository, len(paths))
	for result := range results {
		repositories[result.index] = result.repo
	}
	return repositories
}

func (g *GitService) Inspect(path string) (Repository, error) {
	if !g.HasGit() {
		return Repository{}, errors.New("git executable was not found in PATH")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return Repository{}, err
	}
	revParse, stderr, err := g.runGit(path, g.commandTimeout, "rev-parse", "--show-toplevel", "--short", "HEAD")
	if err != nil {
		topLevel, topLevelStderr, topLevelErr := g.runGit(path, g.commandTimeout, "rev-parse", "--show-toplevel")
		if topLevelErr != nil {
			return Repository{}, fmt.Errorf("not a git repository: %s", firstNonEmpty(topLevelStderr, stderr, err.Error()))
		}
		revParse = topLevel
	}
	revParseLines := splitLines(revParse)
	if len(revParseLines) == 0 {
		return Repository{}, fmt.Errorf("not a git repository: %s", firstNonEmpty(stderr, "empty git rev-parse output"))
	}
	path = filepath.Clean(strings.TrimSpace(revParseLines[0]))
	head := ""
	if len(revParseLines) > 1 {
		head = strings.TrimSpace(revParseLines[1])
	}

	statusOutput, stderr, err := g.runGit(path, g.commandTimeout, "status", "--porcelain=v1", "-uall", "-b", "--ahead-behind")
	if err != nil {
		return Repository{}, fmt.Errorf("git status failed: %s", firstNonEmpty(stderr, err.Error()))
	}
	status, branch, upstream, ahead, behind := parseStatus(statusOutput)
	if branch == "" {
		branch = "HEAD detached"
	}
	remoteURL, _, _ := g.runGit(path, g.commandTimeout, "remote", "get-url", "origin")

	return Repository{
		ID:          repoID(path),
		Name:        filepath.Base(path),
		Path:        path,
		Branch:      branch,
		Head:        head,
		Upstream:    strings.TrimSpace(upstream),
		RemoteURL:   strings.TrimSpace(remoteURL),
		HasUpstream: strings.TrimSpace(upstream) != "",
		IsClean:     len(status.Files) == 0,
		Ahead:       ahead,
		Behind:      behind,
		Status:      status,
		LastCommit:  g.lastCommit(path),
		InspectedAt: nowISO(),
	}, nil
}

func (g *GitService) resolveRepoPath(path string) (string, error) {
	if !g.HasGit() {
		return "", errors.New("git executable was not found in PATH")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	topLevel, stderr, err := g.runGit(path, g.commandTimeout, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %s", firstNonEmpty(stderr, err.Error()))
	}
	return filepath.Clean(strings.TrimSpace(topLevel)), nil
}

func (g *GitService) repositoryStatus(path string) (string, RepoStatus, error) {
	repoPath, err := g.resolveRepoPath(path)
	if err != nil {
		return "", RepoStatus{}, err
	}
	statusOutput, stderr, err := g.runGit(repoPath, g.commandTimeout, "status", "--porcelain=v1", "-uall", "-b", "--ahead-behind")
	if err != nil {
		return "", RepoStatus{}, fmt.Errorf("git status failed: %s", firstNonEmpty(stderr, err.Error()))
	}
	status, _, _, _, _ := parseStatus(statusOutput)
	return repoPath, status, nil
}

func (g *GitService) Branches(path string) (BranchResponse, error) {
	repoPath, err := g.resolveRepoPath(path)
	if err != nil {
		return BranchResponse{}, err
	}
	output, stderr, err := g.runGit(repoPath, g.commandTimeout, "for-each-ref", "--format=%(refname)%00%(refname:short)%00%(HEAD)%00%(upstream:short)%00%(objectname:short)%00%(committerdate:relative)%00%(contents:subject)", "refs/heads", "refs/remotes")
	if err != nil {
		return BranchResponse{}, fmt.Errorf("git branches failed: %s", firstNonEmpty(stderr, err.Error()))
	}
	branches, current := parseBranchRefs(output)
	sort.SliceStable(branches, func(i, j int) bool {
		if branches[i].Current != branches[j].Current {
			return branches[i].Current
		}
		if branches[i].Remote != branches[j].Remote {
			return !branches[i].Remote
		}
		return strings.ToLower(branches[i].Name) < strings.ToLower(branches[j].Name)
	})
	return BranchResponse{Path: repoPath, Current: current, Branches: branches, Generated: nowISO()}, nil
}

func (g *GitService) CheckoutBranch(path string, branch string) (CommandResult, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return CommandResult{}, errors.New("branch is required")
	}
	repoPath, err := g.resolveRepoPath(path)
	if err != nil {
		return CommandResult{}, err
	}
	args := []string{"checkout", branch}
	stdout, stderr, err := g.runGit(repoPath, 2*g.commandTimeout, args...)
	result := CommandResult{Path: repoPath, Command: "git " + strings.Join(args, " "), Success: err == nil, Stdout: stdout, Stderr: stderr, FinishedAt: nowISO()}
	if err != nil {
		result.Message = firstNonEmpty(stderr, err.Error())
		return result, nil
	}
	result.Message = firstNonEmpty(stdout, stderr, "branch checked out")
	return result, nil
}

func (g *GitService) Diff(path string, mode string) (DiffResponse, error) {
	repoPath, status, err := g.repositoryStatus(path)
	if err != nil {
		return DiffResponse{}, err
	}
	mode = normalizeDiffMode(mode)
	args := []string{"diff", "--find-renames", "--find-copies", "--no-ext-diff", "--unified=4"}
	switch mode {
	case diffModeStaged:
		args = append(args, "--cached")
	case diffModeHead:
		args = append(args, "HEAD")
	}
	raw, stderr, err := g.runGit(repoPath, 2*g.commandTimeout, args...)
	response := DiffResponse{Path: repoPath, Mode: mode, Generated: nowISO()}
	if err != nil {
		response.Error = firstNonEmpty(stderr, err.Error())
		return response, nil
	}
	if len(raw) > g.diffByteLimit {
		raw = raw[:g.diffByteLimit] + "\n\n[diff output truncated by Fusion Git Desk]\n"
		response.Truncated = true
	}
	response.Raw = raw
	response.Files = parseUnifiedDiff(raw)
	if mode == diffModeWorking {
		untrackedDiffs, untrackedTruncated := g.untrackedDiffFiles(repoPath, untrackedPaths(status.Files), maxUntrackedFullDiffFiles)
		response.Files = append(response.Files, untrackedDiffs...)
		if untrackedTruncated {
			response.Truncated = true
			response.Note = "Only the first untracked files are shown. Select a file on the right to inspect it directly."
		}
	}
	if len(response.Files) == 0 {
		response.Note = emptyDiffNote(mode, "")
	}
	return response, nil
}

func (g *GitService) FileDiff(path string, mode string, filePath string) (DiffResponse, error) {
	repoPath, status, err := g.repositoryStatus(path)
	if err != nil {
		return DiffResponse{}, err
	}
	filePath = strings.TrimSpace(filepath.ToSlash(filePath))
	if filePath == "" {
		return g.Diff(repoPath, mode)
	}
	mode = normalizeDiffMode(mode)
	response := DiffResponse{Path: repoPath, Mode: mode, Target: filePath, Generated: nowISO()}
	changedFile := findChangedFile(status.Files, filePath)
	if mode == diffModeWorking && changedFile != nil && isUntrackedStatus(changedFile.Status) {
		diffFile, truncated, readErr := g.untrackedDiffFile(repoPath, changedFile.Path)
		if readErr != nil {
			response.Error = readErr.Error()
			return response, nil
		}
		response.Files = []DiffFile{diffFile}
		response.Truncated = truncated
		if truncated {
			response.Note = "This untracked file is large, so only a preview is shown."
		}
		return response, nil
	}
	args := []string{"diff", "--find-renames", "--find-copies", "--no-ext-diff", "--unified=4"}
	switch mode {
	case diffModeStaged:
		args = append(args, "--cached")
	case diffModeHead:
		args = append(args, "HEAD")
	}
	args = append(args, "--", filePath)
	raw, stderr, err := g.runGit(repoPath, 2*g.commandTimeout, args...)
	if err != nil {
		response.Error = firstNonEmpty(stderr, err.Error())
		return response, nil
	}
	if len(raw) > g.diffByteLimit {
		raw = raw[:g.diffByteLimit] + "\n\n[diff output truncated by Fusion Git Desk]\n"
		response.Truncated = true
	}
	response.Raw = raw
	response.Files = parseUnifiedDiff(raw)
	if len(response.Files) == 0 {
		response.Note = emptyDiffNote(mode, filePath)
	}
	return response, nil
}

func (g *GitService) UpdateRepositories(request UpdateRequest) []UpdateResult {
	mode := normalizeUpdateMode(request.Mode)
	paths := uniqueStrings(request.Paths)
	if len(paths) == 0 {
		return nil
	}

	results := make([]UpdateResult, len(paths))
	jobs := make(chan int)
	workerCount := boundedWorkerCount(len(paths), maxUpdateWorkers)
	var wg sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				results[index] = g.updateRepository(paths[index], mode, request.OnlyClean, request.Prune)
			}
		}()
	}
	for index := range paths {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	return results
}

func (g *GitService) updateRepository(path string, mode string, onlyClean bool, prune bool) UpdateResult {
	before, err := g.Inspect(path)
	result := UpdateResult{Path: path, Mode: mode, FinishedAt: nowISO()}
	if err != nil {
		result.Message = err.Error()
		return result
	}
	result.Path = before.Path
	beforeCopy := before
	result.Before = &beforeCopy
	if onlyClean && !before.IsClean {
		result.Skipped = true
		result.Message = "skipped because the working tree has local changes"
		return result
	}
	stdout, stderr, err := g.runGit(before.Path, 4*g.commandTimeout, updateArgs(mode, prune)...)
	result.Stdout = stdout
	result.Stderr = stderr
	result.Success = err == nil
	if err != nil {
		result.Message = firstNonEmpty(stderr, err.Error())
	} else {
		result.Message = firstNonEmpty(stdout, stderr, "up to date")
	}
	after, inspectErr := g.Inspect(before.Path)
	if inspectErr == nil {
		afterCopy := after
		result.After = &afterCopy
	}
	result.FinishedAt = nowISO()
	return result
}

func (g *GitService) lastCommit(path string) CommitInfo {
	output, _, err := g.runGit(path, g.commandTimeout, "log", "-1", "--pretty=format:%h%x00%an%x00%cr%x00%s")
	if err != nil || output == "" {
		return CommitInfo{}
	}
	parts := strings.SplitN(output, "\x00", 4)
	for len(parts) < 4 {
		parts = append(parts, "")
	}
	return CommitInfo{Hash: parts[0], Author: parts[1], RelativeTime: parts[2], Subject: parts[3]}
}

func (g *GitService) runGit(repo string, timeout time.Duration, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmdArgs := append([]string{"-c", "core.quotePath=false", "-C", repo}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	configureHiddenCommand(cmd)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), stderr.String(), fmt.Errorf("git command timed out after %s", timeout)
	}
	return strings.TrimRight(stdout.String(), "\r\n"), strings.TrimSpace(stderr.String()), err
}

func shouldSkipDirectory(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".hg", ".svn", "node_modules", "vendor", "dist", "build", "target", ".idea", ".vscode", ".gradle", ".mvn", ".next", ".nuxt", ".turbo", ".cache":
		return true
	default:
		return false
	}
}

func hasGitMarker(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

func directoryDepth(root string, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	return len(strings.Split(rel, string(os.PathSeparator)))
}

func repoID(path string) string {
	return strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
}

var aheadBehindPattern = regexp.MustCompile(`\[(?:ahead (?P<ahead>\d+))?(?:, )?(?:behind (?P<behind>\d+))?\]`)

func parseStatus(output string) (RepoStatus, string, string, int, int) {
	status := RepoStatus{Files: make([]ChangedFile, 0)}
	branch := ""
	upstream := ""
	ahead := 0
	behind := 0
	for _, line := range splitLines(output) {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			branch, upstream, ahead, behind = parseBranchLine(line)
			continue
		}
		if len(line) < 3 {
			continue
		}
		x := rune(line[0])
		y := rune(line[1])
		fileText := strings.TrimSpace(line[3:])
		changedFile := ChangedFile{Path: fileText, Status: strings.TrimSpace(string([]rune{x, y})), Staged: x != ' ' && x != '?', Unstaged: y != ' ' && y != '?'}
		if strings.Contains(fileText, " -> ") {
			parts := strings.SplitN(fileText, " -> ", 2)
			changedFile.OldPath = parts[0]
			changedFile.Path = parts[1]
		}
		countStatus(&status, x, y)
		status.Files = append(status.Files, changedFile)
	}
	return status, branch, upstream, ahead, behind
}

func parseBranchLine(line string) (string, string, int, int) {
	text := strings.TrimPrefix(line, "## ")
	branchAndUpstream := text
	if idx := strings.Index(branchAndUpstream, " ["); idx >= 0 {
		branchAndUpstream = branchAndUpstream[:idx]
	}
	branch := branchAndUpstream
	upstream := ""
	if idx := strings.Index(branchAndUpstream, "..."); idx >= 0 {
		branch = branchAndUpstream[:idx]
		upstream = branchAndUpstream[idx+3:]
	}
	branch = strings.TrimSpace(branch)
	if strings.HasPrefix(branch, "No commits yet on ") {
		branch = strings.TrimPrefix(branch, "No commits yet on ")
	}
	ahead := 0
	behind := 0
	if match := aheadBehindPattern.FindStringSubmatch(text); match != nil {
		names := aheadBehindPattern.SubexpNames()
		for i, value := range match {
			if value == "" {
				continue
			}
			number, _ := strconv.Atoi(value)
			switch names[i] {
			case "ahead":
				ahead = number
			case "behind":
				behind = number
			}
		}
	}
	return branch, strings.TrimSpace(upstream), ahead, behind
}

func countStatus(status *RepoStatus, x rune, y rune) {
	if x == '?' && y == '?' {
		status.Untracked++
		return
	}
	if x != ' ' && x != '?' {
		status.Staged++
	}
	if y != ' ' && y != '?' {
		status.Unstaged++
	}
	for _, code := range []rune{x, y} {
		switch code {
		case 'A':
			status.Added++
		case 'M':
			status.Modified++
		case 'D':
			status.Deleted++
		case 'R':
			status.Renamed++
		case 'C':
			status.Copied++
		case 'U':
			status.Conflicted++
		}
	}
}

func parseBranchRefs(output string) ([]BranchInfo, string) {
	branches := make([]BranchInfo, 0)
	current := ""
	for _, line := range splitLines(output) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 7)
		for len(parts) < 7 {
			parts = append(parts, "")
		}
		refName := strings.TrimSpace(parts[0])
		shortName := strings.TrimSpace(parts[1])
		if refName == "" || shortName == "" || strings.HasSuffix(shortName, "/HEAD") {
			continue
		}
		isRemote := strings.HasPrefix(refName, "refs/remotes/")
		isCurrent := !isRemote && strings.TrimSpace(parts[2]) == "*"
		if isCurrent {
			current = shortName
		}
		branches = append(branches, BranchInfo{
			Name:         shortName,
			Current:      isCurrent,
			Remote:       isRemote,
			Upstream:     strings.TrimSpace(parts[3]),
			Commit:       shortSHA(parts[4]),
			RelativeTime: strings.TrimSpace(parts[5]),
			Subject:      strings.TrimSpace(parts[6]),
		})
	}
	return branches, current
}

func findChangedFile(files []ChangedFile, path string) *ChangedFile {
	path = filepath.ToSlash(path)
	for i := range files {
		if filepath.ToSlash(files[i].Path) == path || filepath.ToSlash(files[i].OldPath) == path {
			return &files[i]
		}
	}
	return nil
}

func untrackedPaths(files []ChangedFile) []string {
	paths := make([]string, 0)
	for _, file := range files {
		if isUntrackedStatus(file.Status) {
			paths = append(paths, file.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

func isUntrackedStatus(status string) bool {
	return strings.TrimSpace(status) == "??"
}

func (g *GitService) untrackedDiffFiles(repoPath string, paths []string, limit int) ([]DiffFile, bool) {
	if limit <= 0 {
		limit = maxUntrackedFullDiffFiles
	}
	truncated := len(paths) > limit
	if len(paths) > limit {
		paths = paths[:limit]
	}
	files := make([]DiffFile, 0, len(paths))
	for _, path := range paths {
		file, fileTruncated, err := g.untrackedDiffFile(repoPath, path)
		if err != nil {
			files = append(files, DiffFile{NewPath: path, Status: "untracked", Lines: []DiffLine{{Kind: "meta", Content: err.Error()}}})
			continue
		}
		if fileTruncated {
			truncated = true
		}
		files = append(files, file)
	}
	return files, truncated
}

func (g *GitService) untrackedDiffFile(repoPath string, path string) (DiffFile, bool, error) {
	cleanPath := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(cleanPath) {
		return DiffFile{}, false, fmt.Errorf("absolute file paths are not allowed: %s", path)
	}
	fullPath := filepath.Join(repoPath, cleanPath)
	rel, err := filepath.Rel(repoPath, fullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return DiffFile{}, false, fmt.Errorf("file is outside repository: %s", path)
	}
	info, err := os.Lstat(fullPath)
	if err != nil {
		return DiffFile{}, false, err
	}
	diffFile := DiffFile{NewPath: filepath.ToSlash(path), Status: "untracked", Lines: make([]DiffLine, 0)}
	if info.IsDir() {
		diffFile.Lines = append(diffFile.Lines, DiffLine{Kind: "meta", Content: "Directory is untracked. Scan with -uall should normally list its files."})
		return diffFile, false, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		diffFile.Lines = append(diffFile.Lines, DiffLine{Kind: "meta", Content: "Symbolic link preview is not displayed."})
		return diffFile, false, nil
	}
	if !info.Mode().IsRegular() {
		diffFile.Lines = append(diffFile.Lines, DiffLine{Kind: "meta", Content: "Only regular text files are displayed."})
		return diffFile, false, nil
	}
	content, truncated, err := readPreviewFile(fullPath, maxUntrackedPreviewBytes)
	if err != nil {
		return DiffFile{}, false, err
	}
	if isBinaryPreview(content) {
		diffFile.Lines = append(diffFile.Lines, DiffLine{Kind: "meta", Content: "Binary file is not displayed."})
		return diffFile, false, nil
	}
	lines := strings.Split(normalizeNewlines(string(content)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	diffFile.Additions = len(lines)
	diffFile.Lines = append(diffFile.Lines, DiffLine{Kind: "hunk", Content: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines))})
	for index, line := range lines {
		diffFile.Lines = append(diffFile.Lines, DiffLine{Kind: "add", Content: line, NewLine: index + 1})
	}
	if truncated {
		diffFile.Lines = append(diffFile.Lines, DiffLine{Kind: "meta", Content: "[file preview truncated]"})
	}
	return diffFile, truncated, nil
}

func readPreviewFile(path string, limit int) ([]byte, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	buffer := make([]byte, limit+1)
	n, err := io.ReadFull(file, buffer)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, false, err
	}
	truncated := n > limit
	if truncated {
		n = limit
	}
	return buffer[:n], truncated, nil
}

func isBinaryPreview(content []byte) bool {
	for _, value := range content {
		if value == 0 {
			return true
		}
	}
	return false
}

func normalizeNewlines(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

func emptyDiffNote(mode string, target string) string {
	if target != "" {
		return fmt.Sprintf("Git reported %s as changed, but returned no textual diff for this %s view. It may be a line-ending, file-mode, binary, or already-staged/unstaged-only change.", target, mode)
	}
	return fmt.Sprintf("No textual diff was returned for the %s view. Untracked files, line-ending-only changes, or staged-only changes may need a file-specific view.", mode)
}

func shortSHA(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func normalizeDiffMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case diffModeStaged:
		return diffModeStaged
	case diffModeHead:
		return diffModeHead
	default:
		return diffModeWorking
	}
}

func normalizeUpdateMode(mode string) string {
	if strings.ToLower(strings.TrimSpace(mode)) == "pull" {
		return "pull"
	}
	return "fetch"
}

func updateArgs(mode string, prune bool) []string {
	if mode == "pull" {
		return []string{"pull", "--ff-only"}
	}
	if prune {
		return []string{"fetch", "--all", "--prune"}
	}
	return []string{"fetch", "--all"}
}

var hunkPattern = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

func parseUnifiedDiff(raw string) []DiffFile {
	raw = normalizeNewlines(raw)
	files := make([]DiffFile, 0)
	var current *DiffFile
	oldLine := 0
	newLine := 0
	flush := func() {
		if current != nil {
			files = append(files, *current)
			current = nil
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			oldPath, newPath := parseDiffGitLine(line)
			current = &DiffFile{OldPath: oldPath, NewPath: newPath, Status: "modified", Lines: make([]DiffLine, 0)}
			continue
		}
		if current == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "new file mode"):
			current.Status = "added"
			current.Lines = append(current.Lines, DiffLine{Kind: "meta", Content: line})
		case strings.HasPrefix(line, "deleted file mode"):
			current.Status = "deleted"
			current.Lines = append(current.Lines, DiffLine{Kind: "meta", Content: line})
		case strings.HasPrefix(line, "rename from "):
			current.Status = "renamed"
			current.OldPath = strings.TrimPrefix(line, "rename from ")
			current.Lines = append(current.Lines, DiffLine{Kind: "meta", Content: line})
		case strings.HasPrefix(line, "rename to "):
			current.NewPath = strings.TrimPrefix(line, "rename to ")
			current.Lines = append(current.Lines, DiffLine{Kind: "meta", Content: line})
		case strings.HasPrefix(line, "--- "):
			current.OldPath = cleanDiffPath(strings.TrimPrefix(line, "--- "))
			current.Lines = append(current.Lines, DiffLine{Kind: "meta", Content: line})
		case strings.HasPrefix(line, "+++ "):
			current.NewPath = cleanDiffPath(strings.TrimPrefix(line, "+++ "))
			current.Lines = append(current.Lines, DiffLine{Kind: "meta", Content: line})
		case strings.HasPrefix(line, "@@ "):
			match := hunkPattern.FindStringSubmatch(line)
			if len(match) >= 3 {
				oldLine, _ = strconv.Atoi(match[1])
				newLine, _ = strconv.Atoi(match[2])
			}
			current.Lines = append(current.Lines, DiffLine{Kind: "hunk", Content: line})
		case strings.HasPrefix(line, "+"):
			current.Additions++
			current.Lines = append(current.Lines, DiffLine{Kind: "add", Content: line[1:], NewLine: newLine})
			newLine++
		case strings.HasPrefix(line, "-"):
			current.Deletions++
			current.Lines = append(current.Lines, DiffLine{Kind: "delete", Content: line[1:], OldLine: oldLine})
			oldLine++
		case strings.HasPrefix(line, " "):
			current.Lines = append(current.Lines, DiffLine{Kind: "context", Content: line[1:], OldLine: oldLine, NewLine: newLine})
			oldLine++
			newLine++
		case strings.HasPrefix(line, "\\"):
			current.Lines = append(current.Lines, DiffLine{Kind: "meta", Content: line})
		case strings.TrimSpace(line) != "":
			current.Lines = append(current.Lines, DiffLine{Kind: "meta", Content: line})
		}
	}
	flush()
	return files
}

func parseDiffGitLine(line string) (string, string) {
	if rest, ok := strings.CutPrefix(line, "diff --git "); ok {
		if oldPath, newPath, split := splitDiffGitPaths(rest); split {
			return cleanDiffPath(oldPath), cleanDiffPath(newPath)
		}
	}
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return "", ""
	}
	return cleanDiffPath(fields[2]), cleanDiffPath(fields[3])
}

func splitDiffGitPaths(value string) (string, string, bool) {
	if !strings.HasPrefix(value, "a/") {
		return "", "", false
	}
	index := strings.Index(value, " b/")
	if index < 0 {
		return "", "", false
	}
	return value[:index], strings.TrimSpace(value[index+1:]), true
}

func cleanDiffPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	if path == "/dev/null" {
		return ""
	}
	return path
}

func splitLines(output string) []string {
	output = normalizeNewlines(output)
	output = strings.TrimRight(output, "\n")
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, strings.TrimSpace(value))
	}
	return result
}

func boundedWorkerCount(items int, maxWorkers int) int {
	if items <= 0 {
		return 0
	}
	cpuWorkers := runtime.NumCPU()
	if cpuWorkers < 2 {
		cpuWorkers = 2
	}
	if cpuWorkers > maxWorkers {
		cpuWorkers = maxWorkers
	}
	if items < cpuWorkers {
		return items
	}
	return cpuWorkers
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}
