package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFile(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantTitle string
		wantTags  []string
		wantErr   bool
	}{
		{
			name:      "basic",
			content:   "---\ntitle: Go 规范\ntags:\n  - 场景:开发\n---\n正文内容",
			wantTitle: "Go 规范",
			wantTags:  []string{"场景:开发"},
		},
		{
			name:    "missing front matter",
			content: "no delimiter here",
			wantErr: true,
		},
		{
			name:    "missing closing delimiter",
			content: "---\ntitle: t\n",
			wantErr: true,
		},
		{
			name:    "invalid yaml",
			content: "---\ntitle: [unclosed\n---\nbody",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "doc.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			doc, err := ParseFile(path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got doc %+v", doc)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if doc.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", doc.Title, tt.wantTitle)
			}
			if len(doc.Tags) != len(tt.wantTags) || doc.Tags[0] != tt.wantTags[0] {
				t.Errorf("Tags = %v, want %v", doc.Tags, tt.wantTags)
			}
			if doc.FilePath != path {
				t.Errorf("FilePath = %q, want %q", doc.FilePath, path)
			}
		})
	}
}

func TestWriteDraft(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "drafts"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := WriteDraft(dir, "K8s 部署踩坑", "## 问题\nPod 起不来", []string{"场景:部署", "域:K8s", "类型:避坑"})
	if err != nil {
		t.Fatalf("WriteDraft: %v", err)
	}

	if !strings.HasPrefix(filepath.Base(path), "2026-") {
		t.Errorf("filename %q should start with date prefix", path)
	}
	if !strings.HasSuffix(path, ".yaml") {
		t.Errorf("filename %q should end with .yaml", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"title: K8s 部署踩坑", "status: pending_review", "域:K8s", "## 问题"} {
		if !strings.Contains(content, want) {
			t.Errorf("draft content missing %q:\n%s", want, content)
		}
	}
}

func TestStoreReloadExcludesNonActive(t *testing.T) {
	root := t.TempDir()
	devDir := filepath.Join(root, "dev", "common")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(name, status string) {
		content := "---\ntitle: " + name + "\nstatus: " + status + "\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(devDir, name+".yaml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("active-doc", "active")
	write("archived-doc", "archived")
	write("pending-doc", "pending_review")
	write("rejected-doc", "rejected")

	s := NewStore(root)
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	docs := s.AllDocs()
	if len(docs) != 1 {
		t.Fatalf("expected 1 active doc, got %d", len(docs))
	}
	if docs[0].Title != "active-doc" {
		t.Errorf("got %q, want active-doc", docs[0].Title)
	}
}

func TestApproveDraft(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	if err := os.MkdirAll(s.DraftsDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	// domain dir pre-exists so approval routes there
	k8sDir := filepath.Join(root, "dev", "k8s")
	if err := os.MkdirAll(k8sDir, 0o755); err != nil {
		t.Fatal(err)
	}

	draftPath, err := WriteDraft(s.DraftsDir(), "K8s 部署踩坑", "## 问题\nPod 起不来", []string{"场景:部署", "域:K8s", "类型:避坑"})
	if err != nil {
		t.Fatal(err)
	}

	doc, err := ParseFile(draftPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveDraft(doc); err != nil {
		t.Fatalf("ApproveDraft: %v", err)
	}

	entries, err := os.ReadDir(k8sDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in domain dir, got %d", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(k8sDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "status: active") {
		t.Errorf("approved doc should have status active:\n%s", data)
	}
	if _, err := os.Stat(draftPath); !os.IsNotExist(err) {
		t.Errorf("draft %q should be removed after approval", draftPath)
	}
}

func TestApproveDraftDefaultsToCommon(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	if err := os.MkdirAll(s.DraftsDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	draftPath, err := WriteDraft(s.DraftsDir(), "通用规范", "内容", []string{"场景:开发", "类型:规范"})
	if err != nil {
		t.Fatal(err)
	}

	doc, err := ParseFile(draftPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveDraft(doc); err != nil {
		t.Fatalf("ApproveDraft: %v", err)
	}

	commonDir := filepath.Join(root, "dev", "common")
	entries, err := os.ReadDir(commonDir)
	if err != nil {
		t.Fatalf("common dir should exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in common dir, got %d", len(entries))
	}
}

func TestRejectDraft(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	if err := os.MkdirAll(s.DraftsDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	draftPath, err := WriteDraft(s.DraftsDir(), "废弃草稿", "内容", []string{"场景:开发", "类型:规范"})
	if err != nil {
		t.Fatal(err)
	}

	doc, err := ParseFile(draftPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RejectDraft(doc); err != nil {
		t.Fatalf("RejectDraft: %v", err)
	}
	if _, err := os.Stat(draftPath); !os.IsNotExist(err) {
		t.Errorf("draft %q should be removed", draftPath)
	}
}
