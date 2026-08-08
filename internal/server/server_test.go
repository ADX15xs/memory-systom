package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"knowledge-base/internal/fs"
)

func TestResolveDraftPath(t *testing.T) {
	root := t.TempDir()
	draftsDir := filepath.Join(root, "drafts")
	if err := os.MkdirAll(draftsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	validDraft := filepath.Join(draftsDir, "note.yaml")
	if err := os.WriteFile(validDraft, []byte("---\ntitle: t\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}

	outsideFile := filepath.Join(root, "secret.yaml")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Server{store: fs.NewStore(root)}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid draft", validDraft, false},
		{"nested draft", filepath.Join(draftsDir, "sub", "x.yaml"), false},
		{"empty path", "", true},
		{"traversal escapes root", filepath.Join(draftsDir, "..", "..", "etc", "passwd"), true},
		{"sibling file outside drafts", outsideFile, true},
		{"drafts dir itself", draftsDir, true},
		{"relative traversal", "../../../../etc/passwd", true},
		{"prefix lookalike dir", root + string(filepath.Separator) + "drafts-evil" + string(filepath.Separator) + "x.yaml", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.resolveDraftPath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got path %q", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if !strings.HasPrefix(got, draftsDir) {
				t.Fatalf("resolved path %q escaped drafts dir %q", got, draftsDir)
			}
		})
	}
}

func TestResolveDraftPathRejectsDeletionOfOutsideFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "drafts"), 0o755); err != nil {
		t.Fatal(err)
	}

	victim := filepath.Join(root, "important.txt")
	if err := os.WriteFile(victim, []byte("do not delete"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Server{store: fs.NewStore(root)}

	if _, err := s.resolveDraftPath(victim); err == nil {
		t.Fatal("resolveDraftPath accepted a file outside drafts/; traversal guard is broken")
	}

	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("victim file should still exist: %v", err)
	}
}
