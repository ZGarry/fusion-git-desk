package main

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseStatusKeepsNestedUntrackedFiles(t *testing.T) {
	status, _, _, _, _ := parseStatus("## master\n?? AGENTS.md\n?? desktop/fusion-git-desk/app.go\n")

	if status.Untracked != 2 {
		t.Fatalf("expected 2 untracked files, got %d", status.Untracked)
	}
	if len(status.Files) != 2 {
		t.Fatalf("expected 2 changed files, got %d", len(status.Files))
	}
	if status.Files[1].Path != "desktop/fusion-git-desk/app.go" {
		t.Fatalf("unexpected nested path: %s", status.Files[1].Path)
	}
}

func TestParseStatusReadsUpstreamAheadBehind(t *testing.T) {
	_, branch, upstream, ahead, behind := parseStatus("## main...origin/main [ahead 2, behind 3]\n M app.go\n")

	if branch != "main" {
		t.Fatalf("unexpected branch: %s", branch)
	}
	if upstream != "origin/main" {
		t.Fatalf("unexpected upstream: %s", upstream)
	}
	if ahead != 2 || behind != 3 {
		t.Fatalf("unexpected ahead/behind: %d/%d", ahead, behind)
	}
}

func TestParseBranchRefsMarksDefaultAndIgnoresAIReview(t *testing.T) {
	output := strings.Join([]string{
		"refs/heads/main\x00main\x00*\x00origin/main\x006b1054bc\x002 hours ago\x00local main\x00",
		"refs/remotes/origin/HEAD\x00origin/HEAD\x00\x00\x006b1054bc\x002 hours ago\x00\x00origin/main",
		"refs/remotes/origin/main\x00origin/main\x00\x00\x006b1054bc\x002 hours ago\x00remote main\x00",
		"refs/remotes/origin/feature/work\x00origin/feature/work\x00\x00\x002c0f9f1a\x001 day ago\x00feature work\x00",
		"refs/remotes/origin/ai-review/bug_detection/foo\x00origin/ai-review/bug_detection/foo\x00\x00\x00f637512d\x001 day ago\x00ai review\x00",
	}, "\n")

	branches, current := parseBranchRefs(output)

	if current != "main" {
		t.Fatalf("unexpected current branch: %s", current)
	}
	var foundDefault bool
	for _, branch := range branches {
		if strings.Contains(branch.Name, "ai-review") {
			t.Fatalf("ai-review branch should be ignored: %#v", branch)
		}
		if branch.Name == "origin/main" {
			foundDefault = branch.Default
		}
	}
	if !foundDefault {
		t.Fatalf("expected origin/main to be marked as default: %#v", branches)
	}
}

func TestParseStatusCountsUnmergedConflictsOnce(t *testing.T) {
	status, _, _, _, _ := parseStatus("## main\nUU both-modified.txt\nAA both-added.txt\nDD both-deleted.txt\nAU added-by-us.txt\nUD deleted-by-them.txt\n")

	if status.Conflicted != 5 {
		t.Fatalf("expected 5 conflicted files, got %d", status.Conflicted)
	}
	if status.Staged != 0 || status.Unstaged != 0 {
		t.Fatalf("unmerged files should not count as staged or unstaged: %#v", status)
	}
	for _, file := range status.Files {
		if file.Staged || file.Unstaged {
			t.Fatalf("unmerged file should not enable staged flags: %#v", file)
		}
	}
}

func TestUntrackedDiffFileRendersTextPreview(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	diffFile, truncated, err := NewGitService().untrackedDiffFile(root, "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("small text file should not be truncated")
	}
	if diffFile.Status != "untracked" {
		t.Fatalf("unexpected status: %s", diffFile.Status)
	}
	if diffFile.Additions != 2 {
		t.Fatalf("expected 2 rendered additions, got %d", diffFile.Additions)
	}
	if len(diffFile.Lines) < 3 || diffFile.Lines[1].Content != "one" {
		t.Fatalf("unexpected diff lines: %#v", diffFile.Lines)
	}
}

func TestUntrackedSummaryFilesDoNotReadFileContent(t *testing.T) {
	files, truncated := untrackedSummaryFiles([]string{"one.txt", "two.txt", "three.txt"}, 2)

	if !truncated {
		t.Fatal("expected summary list to report truncation")
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 summary files, got %d", len(files))
	}
	if files[0].Additions != 0 || len(files[0].Lines) != 1 || files[0].Lines[0].Kind != "meta" {
		t.Fatalf("expected metadata-only summary file, got %#v", files[0])
	}
}

func TestUntrackedDiffFileAllowsDotPrefixedName(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "..notes.txt")
	if err := os.WriteFile(path, []byte("safe\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	diffFile, _, err := NewGitService().untrackedDiffFile(root, "..notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if diffFile.Additions != 1 {
		t.Fatalf("expected preview for dot-prefixed file, got %#v", diffFile)
	}
}

func TestUntrackedDiffFileRejectsParentTraversal(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(outside)
	})

	_, _, err := NewGitService().untrackedDiffFile(root, "../outside.txt")
	if err == nil {
		t.Fatal("expected parent traversal to be rejected")
	}
}

func TestUntrackedDiffFileDoesNotFollowSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(filepath.Dir(root), "outside-target.txt")
	link := filepath.Join(root, "linked.txt")
	if err := os.WriteFile(target, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(target)
	})
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	diffFile, truncated, err := NewGitService().untrackedDiffFile(root, "linked.txt")
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("symlink metadata should not be truncated")
	}
	if diffFile.Additions != 0 || len(diffFile.Lines) != 1 || diffFile.Lines[0].Kind != "meta" {
		t.Fatalf("expected symlink metadata only, got %#v", diffFile)
	}
}

func TestParseDiffGitLineKeepsSpaces(t *testing.T) {
	oldPath, newPath := parseDiffGitLine("diff --git a/folder/old name.txt b/folder/new name.txt")

	if oldPath != "folder/old name.txt" {
		t.Fatalf("unexpected old path: %q", oldPath)
	}
	if newPath != "folder/new name.txt" {
		t.Fatalf("unexpected new path: %q", newPath)
	}
}

func TestDurationMillisRoundsPositiveSubmillisecond(t *testing.T) {
	if got := durationMillis(500 * time.Microsecond); got != 1 {
		t.Fatalf("expected submillisecond duration to round to 1ms, got %d", got)
	}
	if got := durationMillis(1500 * time.Millisecond); got != 1500 {
		t.Fatalf("expected millisecond duration to be preserved, got %d", got)
	}
	if got := durationMillis(0); got != 0 {
		t.Fatalf("expected zero duration to stay zero, got %d", got)
	}
}

func TestResolveEditorLaunchCommandRejectsUnknownEditor(t *testing.T) {
	if _, err := resolveEditorLaunchCommand("unknown-editor", t.TempDir(), ""); err == nil {
		t.Fatal("expected unknown editor to be rejected")
	}
}

func TestResolveEditorLaunchCommandPrefersConfiguredExecutable(t *testing.T) {
	root := t.TempDir()
	editorPath := filepath.Join(root, "idea64.exe")
	if err := os.WriteFile(editorPath, []byte("placeholder"), 0o755); err != nil {
		t.Fatal(err)
	}

	launch, err := resolveEditorLaunchCommand("idea", root, editorPath)
	if err != nil {
		t.Fatal(err)
	}
	if launch.Executable != editorPath {
		t.Fatalf("expected configured executable %q, got %q", editorPath, launch.Executable)
	}
}

func TestFindFirstExecutableAcceptsExistingAbsolutePath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "editor.exe")
	if err := os.WriteFile(path, []byte("placeholder"), 0o755); err != nil {
		t.Fatal(err)
	}

	executable, err := findFirstExecutable([]string{"", filepath.Join(root, "missing.exe"), path})
	if err != nil {
		t.Fatal(err)
	}
	if executable != path {
		t.Fatalf("expected %q, got %q", path, executable)
	}
}

func TestInspectDetectsNonOriginRemote(t *testing.T) {
	root := initTestRepo(t)
	commitTestFile(t, root, "README.md", "hello\n", "initial commit")
	runTestGit(t, root, "remote", "add", "upstream", "https://example.invalid/fusion.git")

	repo, err := NewGitService().Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if !repo.HasRemote {
		t.Fatalf("expected remote to be detected: %#v", repo)
	}
	if repo.RemoteName != "upstream" {
		t.Fatalf("expected upstream remote, got %q", repo.RemoteName)
	}
	if repo.RemoteURL != "https://example.invalid/fusion.git" {
		t.Fatalf("unexpected remote URL: %q", repo.RemoteURL)
	}
	if repo.HasUpstream {
		t.Fatalf("remote existence should not imply branch upstream: %#v", repo)
	}
}

func TestScanFindsNestedRepositoriesAndRemoteState(t *testing.T) {
	workspace := t.TempDir()
	repoA := filepath.Join(workspace, "service-a")
	repoB := filepath.Join(workspace, "group", "service-b")
	if err := os.MkdirAll(repoA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repoB, 0o755); err != nil {
		t.Fatal(err)
	}
	initExistingTestRepo(t, repoA)
	initExistingTestRepo(t, repoB)
	commitTestFile(t, repoA, "README.md", "a\n", "initial commit")
	commitTestFile(t, repoB, "README.md", "b\n", "initial commit")
	runTestGit(t, repoA, "remote", "add", "upstream", "https://example.invalid/service-a.git")
	if err := os.WriteFile(filepath.Join(repoB, "local.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repositories, warnings, err := NewGitService().Scan(workspace, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if len(repositories) != 2 {
		t.Fatalf("expected 2 repositories, got %#v", repositories)
	}

	byName := make(map[string]Repository)
	for _, repo := range repositories {
		byName[repo.Name] = repo
	}
	if !byName["service-a"].HasRemote || byName["service-a"].RemoteName != "upstream" {
		t.Fatalf("expected service-a remote state, got %#v", byName["service-a"])
	}
	if byName["service-a"].HasUpstream {
		t.Fatalf("remote existence should not imply upstream: %#v", byName["service-a"])
	}
	if byName["service-b"].IsClean || byName["service-b"].Status.Untracked != 1 {
		t.Fatalf("expected service-b local change, got %#v", byName["service-b"])
	}
}

func TestScanRepoPathsCollectsWalkWarnings(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	blockedPath := filepath.Join(root, "blocked")
	walkErr := errors.New("access denied")

	paths, warnings, err := scanRepoPaths(root, 5, func(_ string, fn fs.WalkDirFunc) error {
		call := func(path string, entry fs.DirEntry, err error) error {
			callbackErr := fn(path, entry, err)
			if callbackErr != nil && !errors.Is(callbackErr, filepath.SkipDir) {
				return callbackErr
			}
			return nil
		}
		if err := call(root, fakeDirEntry{name: filepath.Base(root), dir: true}, nil); err != nil {
			return err
		}
		if err := call(repoPath, fakeDirEntry{name: "repo", dir: true}, nil); err != nil {
			return err
		}
		return call(blockedPath, nil, walkErr)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != filepath.Clean(repoPath) {
		t.Fatalf("expected discovered repo path, got %#v", paths)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one scan warning, got %#v", warnings)
	}
	if !strings.Contains(warnings[0], filepath.Clean(blockedPath)) || !strings.Contains(warnings[0], "access denied") {
		t.Fatalf("warning should include skipped path and reason, got %q", warnings[0])
	}
}

func TestPullSkipsRepositoryWithoutUpstream(t *testing.T) {
	root := initTestRepo(t)
	commitTestFile(t, root, "README.md", "hello\n", "initial commit")

	result := NewGitService().updateRepository(root, "pull", false, false)

	if !result.Skipped {
		t.Fatalf("expected pull without upstream to be skipped, got %#v", result)
	}
	if result.Success {
		t.Fatalf("skipped pull should not be marked successful: %#v", result)
	}
	if !strings.Contains(result.Message, "远端分支") {
		t.Fatalf("expected remote branch guidance in message, got %q", result.Message)
	}
}

func TestPullSkipsRepositoryWithConflicts(t *testing.T) {
	root := initTestRepo(t)
	commitTestFile(t, root, "README.md", "base\n", "initial commit")
	runTestGit(t, root, "checkout", "-b", "feature")
	commitTestFile(t, root, "README.md", "feature\n", "feature change")
	runTestGit(t, root, "checkout", "main")
	commitTestFile(t, root, "README.md", "main\n", "main change")
	runTestGitExpectError(t, root, "merge", "feature")

	result := NewGitService().updateRepository(root, "pull", false, false)

	if !result.Skipped {
		t.Fatalf("expected conflicted pull to be skipped, got %#v", result)
	}
	if result.Before == nil || result.Before.Status.Conflicted == 0 {
		t.Fatalf("expected conflicted status before pull, got %#v", result.Before)
	}
	if !strings.Contains(result.Message, "冲突") {
		t.Fatalf("expected conflict guidance in message, got %q", result.Message)
	}
}

func TestCheckoutRemoteBranchFetchesAndTracksBranch(t *testing.T) {
	workspace := t.TempDir()
	source := filepath.Join(workspace, "source")
	client := filepath.Join(workspace, "client")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	initExistingTestRepo(t, source)
	commitTestFile(t, source, "README.md", "main\n", "initial commit")
	runTestGit(t, source, "checkout", "-b", "feature/remote")
	commitTestFile(t, source, "feature.txt", "remote\n", "remote feature")
	runTestGit(t, source, "checkout", "main")
	runTestGit(t, workspace, "clone", source, client)

	result, err := NewGitService().CheckoutRemoteBranch(client, "origin/feature/remote")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected remote checkout to succeed: %#v", result)
	}
	current := strings.TrimSpace(runTestGit(t, client, "branch", "--show-current"))
	if current != "feature/remote" {
		t.Fatalf("unexpected current branch: %s", current)
	}
	upstream := strings.TrimSpace(runTestGit(t, client, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"))
	if upstream != "origin/feature/remote" {
		t.Fatalf("unexpected upstream: %s", upstream)
	}
	if _, err := os.Stat(filepath.Join(client, "feature.txt")); err != nil {
		t.Fatalf("expected remote branch file to be checked out: %v", err)
	}
}

func TestStageAndUnstageFile(t *testing.T) {
	root := initTestRepo(t)
	commitTestFile(t, root, "notes.txt", "one\n", "initial commit")
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stageResult, err := NewGitService().StageFile(root, "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !stageResult.Success {
		t.Fatalf("expected stage to succeed: %#v", stageResult)
	}
	_, stagedStatus, err := NewGitService().repositoryStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	if stagedStatus.Staged != 1 || stagedStatus.Unstaged != 0 {
		t.Fatalf("expected staged-only change, got %#v", stagedStatus)
	}

	unstageResult, err := NewGitService().UnstageFile(root, "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !unstageResult.Success {
		t.Fatalf("expected unstage to succeed: %#v", unstageResult)
	}
	_, unstagedStatus, err := NewGitService().repositoryStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	if unstagedStatus.Staged != 0 || unstagedStatus.Unstaged != 1 {
		t.Fatalf("expected unstaged-only change, got %#v", unstagedStatus)
	}
}

func TestCommitRepositoryCommitsStagedFiles(t *testing.T) {
	root := initTestRepo(t)
	commitTestFile(t, root, "notes.txt", "one\n", "initial commit")
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result, err := NewGitService().StageFile(root, "notes.txt"); err != nil || !result.Success {
		t.Fatalf("stage failed: result=%#v err=%v", result, err)
	}

	result, err := NewGitService().CommitRepository(root, "update notes")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected commit to succeed: %#v", result)
	}
	_, status, err := NewGitService().repositoryStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Files) != 0 {
		t.Fatalf("expected clean repo after commit, got %#v", status)
	}
	if subject := runTestGit(t, root, "log", "-1", "--pretty=%s"); strings.TrimSpace(subject) != "update notes" {
		t.Fatalf("unexpected commit subject: %q", subject)
	}
}

func TestCommitRepositoryRequiresStagedFiles(t *testing.T) {
	root := initTestRepo(t)
	commitTestFile(t, root, "notes.txt", "one\n", "initial commit")
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := NewGitService().CommitRepository(root, "update notes")
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatalf("commit without staged files should not succeed: %#v", result)
	}
	if !strings.Contains(result.Message, "暂存") {
		t.Fatalf("expected staged guidance, got %q", result.Message)
	}
}

func TestCommitRepositoryRejectsBothAddedConflict(t *testing.T) {
	root := initTestRepo(t)
	commitTestFile(t, root, "README.md", "base\n", "initial commit")
	runTestGit(t, root, "checkout", "-b", "feature")
	commitTestFile(t, root, "conflict.txt", "feature\n", "feature adds conflict")
	runTestGit(t, root, "checkout", "main")
	commitTestFile(t, root, "conflict.txt", "main\n", "main adds conflict")
	runTestGitExpectError(t, root, "merge", "feature")

	_, status, err := NewGitService().repositoryStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	if status.Conflicted != 1 {
		t.Fatalf("expected add/add conflict to be detected once, got %#v", status)
	}

	result, err := NewGitService().CommitRepository(root, "commit conflicted state")
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatalf("commit with unmerged files should not succeed: %#v", result)
	}
	if !strings.Contains(result.Message, "冲突") {
		t.Fatalf("expected conflict guidance, got %q", result.Message)
	}
}

func TestStageFileRejectsParentTraversal(t *testing.T) {
	root := initTestRepo(t)
	commitTestFile(t, root, "notes.txt", "one\n", "initial commit")

	result, err := NewGitService().StageFile(root, "../outside.txt")
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatalf("expected unsafe path to fail, got %#v", result)
	}
	if !strings.Contains(result.Message, "变更列表") {
		t.Fatalf("expected changed-list guidance, got %q", result.Message)
	}
}

func TestSafeRepoPathspecRejectsParentTraversal(t *testing.T) {
	root := t.TempDir()

	if _, err := safeRepoPathspec(root, "../outside.txt"); err == nil {
		t.Fatal("expected parent traversal to be rejected")
	}
	if _, err := safeRepoPathspec(root, filepath.Join(root, "absolute.txt")); err == nil {
		t.Fatal("expected absolute path to be rejected")
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	initExistingTestRepo(t, root)
	return root
}

func initExistingTestRepo(t *testing.T, root string) {
	t.Helper()
	runTestGit(t, root, "init", "-b", "main")
	runTestGit(t, root, "config", "user.name", "Fusion Git Desk Test")
	runTestGit(t, root, "config", "user.email", "fusion-git-desk-test@example.invalid")
}

func commitTestFile(t *testing.T, root string, name string, content string, message string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, root, "add", name)
	runTestGit(t, root, "commit", "-m", message)
}

func runTestGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	output, err := runTestGitCommand(root, args...)
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return output
}

func runTestGitExpectError(t *testing.T, root string, args ...string) string {
	t.Helper()
	output, err := runTestGitCommand(root, args...)
	if err == nil {
		t.Fatalf("git %v unexpectedly succeeded:\n%s", args, output)
	}
	return output
}

func runTestGitCommand(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	configureHiddenCommand(cmd)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

type fakeDirEntry struct {
	name string
	dir  bool
}

func (e fakeDirEntry) Name() string {
	return e.name
}

func (e fakeDirEntry) IsDir() bool {
	return e.dir
}

func (e fakeDirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	}
	return 0
}

func (e fakeDirEntry) Info() (fs.FileInfo, error) {
	return nil, errors.New("fake entry has no file info")
}
