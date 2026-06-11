package main

import (
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
	if !strings.Contains(result.Message, "upstream") {
		t.Fatalf("expected upstream guidance in message, got %q", result.Message)
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
	runTestGit(t, root, "init", "-b", "main")
	runTestGit(t, root, "config", "user.name", "Fusion Git Desk Test")
	runTestGit(t, root, "config", "user.email", "fusion-git-desk-test@example.invalid")
	return root
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
