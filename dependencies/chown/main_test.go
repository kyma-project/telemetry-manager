package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestRun_InvalidOwnerFormat(t *testing.T) {
	tests := []struct {
		name     string
		ownerArg string
	}{
		{"no colon", "10001"},
		{"empty", ""},
		{"non-numeric uid", "abc:0"},
		{"non-numeric gid", "10001:xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := run(tt.ownerArg, t.TempDir()); err == nil {
				t.Errorf("expected error for owner arg %q, got nil", tt.ownerArg)
			}
		})
	}
}

func TestRun_NonExistentPath(t *testing.T) {
	if err := run("0:0", "/nonexistent/path/that/does/not/exist"); err == nil {
		t.Error("expected error for non-existent path, got nil")
	}
}

func TestRun_WalksRecursively(t *testing.T) {
	dir := t.TempDir()

	// Create a nested structure: dir/sub/file
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(sub, "file.txt")
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Chown to the current process owner — valid, no privilege required.
	uid := strconv.Itoa(os.Getuid())
	gid := strconv.Itoa(os.Getgid())

	if err := run(uid+":"+gid, dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_Symlink(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	uid := strconv.Itoa(os.Getuid())
	gid := strconv.Itoa(os.Getgid())

	// Lchown on a symlink should not follow it — must not error.
	if err := run(uid+":"+gid, dir); err != nil {
		t.Fatalf("unexpected error on symlink: %v", err)
	}
}
