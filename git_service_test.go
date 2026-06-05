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
