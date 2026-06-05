package main

import (
	"os"
	"path/filepath"
	"testing"
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
