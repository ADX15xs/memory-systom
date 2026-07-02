// Package fs handles file system operations for the knowledge base:
// reading YAML documents, writing drafts, and scanning directories.
package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// --- Data Model ---

// Document represents a single knowledge document.
type Document struct {
	Title    string   `yaml:"title"`
	Tags     []string `yaml:"tags"`
	Status   string   `yaml:"status,omitempty"`
	FilePath string   // absolute path
	Content  string   // Markdown body (everything after first ---)
	Score    float64  // populated by search engine
}

// FrontMatter is only the YAML header fields, used during parsing.
type FrontMatter struct {
	Title  string   `yaml:"title"`
	Tags   []string `yaml:"tags"`
	Status string   `yaml:"status,omitempty"`
}

// ParseFile reads a YAML knowledge file and returns a Document.
func ParseFile(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	content := string(data)

	// Find the first YAML front matter delimiter ---
	const delim = "---"
	if !strings.HasPrefix(content, delim) {
		return nil, fmt.Errorf("file %s: missing YAML front matter (must start with ---)", path)
	}

	parts := strings.SplitN(content[3:], delim, 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("file %s: missing closing --- for front matter", path)
	}

	var fm FrontMatter
	if err := yaml.Unmarshal([]byte(parts[0]), &fm); err != nil {
		return nil, fmt.Errorf("file %s: parse YAML front matter: %w", path, err)
	}

	body := strings.TrimSpace(parts[1])

	return &Document{
		Title:    fm.Title,
		Tags:     fm.Tags,
		Status:   fm.Status,
		FilePath: path,
		Content:  body,
	}, nil
}

// WriteDraft writes a new draft file to the drafts directory.
// File name: {date}_{slug}.yaml
func WriteDraft(draftsDir string, title, content string, tags []string) (string, error) {
	slug := slugify(title)
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s_%s.yaml", date, slug)
	path := filepath.Join(draftsDir, filename)

	var buf strings.Builder
	buf.WriteString("---\n")
	fmt.Fprintf(&buf, "title: %s\n", yamlString(title))
	fmt.Fprintf(&buf, "status: pending_review\n")
	fmt.Fprintf(&buf, "created_at: %s\n", time.Now().Format(time.RFC3339))
	if len(tags) > 0 {
		buf.WriteString("tags:\n")
		for _, t := range tags {
			fmt.Fprintf(&buf, "  - %s\n", yamlString(t))
		}
	}
	buf.WriteString("---\n")
	buf.WriteString(content)
	buf.WriteByte('\n')

	if err := os.WriteFile(path, []byte(buf.String()), 0644); err != nil {
		return "", fmt.Errorf("write draft %s: %w", path, err)
	}
	return path, nil
}

// yamlString safely quotes a YAML string if needed.
func yamlString(s string) string {
	if strings.ContainsAny(s, ":[]{}#&*!|>'\"%@`") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, " ") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

func slugify(title string) string {
	slug := strings.ToLower(title)
	// Replace spaces/tabs with hyphens, keep alphanumeric, drop other ASCII punctuation
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		if r == ' ' || r == '\t' {
			return '-'
		}
		// Keep Chinese characters (CJK Unified Ideographs)
		if r >= 0x4e00 && r <= 0x9fff {
			return r
		}
		return -1
	}, slug)
	// Collapse hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		// Fallback: use first 20 chars of title with non-ascii replaced
		for _, r := range title {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
				slug += string(r)
			} else {
				slug += "-"
			}
		}
		slug = strings.Trim(slug, "-")
		for strings.Contains(slug, "--") {
			slug = strings.ReplaceAll(slug, "--", "-")
		}
		if slug == "" {
			slug = "draft"
		}
		if len(slug) > 20 {
			slug = slug[:20]
		}
	}
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return slug
}

// --- Knowledge Store ---

// Store holds the in-memory document collection.
type Store struct {
	mu            sync.RWMutex
	docs          []*Document
	rootDir       string
	draftsDir     string
	onReload      func() // callback after hot reload
}

// NewStore creates an empty store.
func NewStore(rootDir string) *Store {
	return &Store{
		rootDir:   rootDir,
		draftsDir: filepath.Join(rootDir, "drafts"),
	}
}

// SetReloadCallback registers a function called after each hot reload.
func (s *Store) SetReloadCallback(fn func()) {
	s.onReload = fn
}

// RootDir returns the knowledge base root directory.
func (s *Store) RootDir() string { return s.rootDir }

// DraftsDir returns the drafts directory path.
func (s *Store) DraftsDir() string { return s.draftsDir }

// Reload rescans the active knowledge directories and rebuilds the document list.
// Documents with status "archived" or "pending_review" are excluded.
// Documents in the archive/ directory are excluded.
func (s *Store) Reload() error {
	docs := make([]*Document, 0)

	// Scan dev/ directory (the only domain in MVP)
	devDir := filepath.Join(s.rootDir, "dev")
	if info, err := os.Stat(devDir); err == nil && info.IsDir() {
		err = filepath.WalkDir(devDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(path), ".yaml") {
				return nil
			}
			doc, err := ParseFile(path)
			if err != nil {
				// Skip unparseable files but log them
				return nil
			}
			// Skip archived or pending_review documents
			if doc.Status == "archived" || doc.Status == "pending_review" || doc.Status == "rejected" {
				return nil
			}
			// Default status to active
			if doc.Status == "" {
				doc.Status = "active"
			}
			docs = append(docs, doc)
			return nil
		})
		if err != nil {
			return fmt.Errorf("walk %s: %w", devDir, err)
		}
	}

	s.mu.Lock()
	s.docs = docs
	s.mu.Unlock()

	if s.onReload != nil {
		s.onReload()
	}
	return nil
}

// AllDocs returns a copy of all currently loaded documents.
func (s *Store) AllDocs() []*Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Document, len(s.docs))
	copy(out, s.docs)
	return out
}

// UpdateFile replaces a document's file content and triggers a reload.
func (s *Store) UpdateFile(path, newContent string) error {
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return err
	}
	return s.Reload()
}

// --- Draft operations ---

// ListDrafts returns all draft documents from the drafts directory.
func (s *Store) ListDrafts() ([]*Document, error) {
	drafts := make([]*Document, 0)
	if info, err := os.Stat(s.draftsDir); err != nil || !info.IsDir() {
		return drafts, nil
	}

	entries, err := os.ReadDir(s.draftsDir)
	if err != nil {
		return nil, fmt.Errorf("read drafts dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".yaml") {
			continue
		}
		path := filepath.Join(s.draftsDir, e.Name())
		doc, err := ParseFile(path)
		if err != nil {
			continue // skip unparseable
		}
		drafts = append(drafts, doc)
	}

	sort.Slice(drafts, func(i, j int) bool {
		return drafts[i].FilePath < drafts[j].FilePath
	})
	return drafts, nil
}

// ApproveDraft moves a draft from drafts/ to the appropriate knowledge directory.
// If no target tag of the form "域:<name>" exists, it goes to dev/common/.
func (s *Store) ApproveDraft(doc *Document) error {
	if doc.Status != "pending_review" && doc.Status != "" {
		doc.Status = "pending_review"
	}

	// Determine target subdirectory based on tags
	targetDir := filepath.Join(s.rootDir, "dev", "common")
	for _, t := range doc.Tags {
		if strings.HasPrefix(t, "域:") {
			domain := strings.TrimPrefix(t, "域:")
			candidate := filepath.Join(s.rootDir, "dev", domain)
			if info, _ := os.Stat(candidate); info != nil && info.IsDir() {
				targetDir = candidate
				break
			}
		}
	}

	// Create target if not exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	// Read current file content
	data, err := os.ReadFile(doc.FilePath)
	if err != nil {
		return fmt.Errorf("read draft: %w", err)
	}

	// Update status from pending_review to active
	content := string(data)
	content = strings.Replace(content, "status: pending_review", "status: active", 1)

	destPath := filepath.Join(targetDir, filepath.Base(doc.FilePath))
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write to target: %w", err)
	}

	// Remove draft
	if err := os.Remove(doc.FilePath); err != nil {
		return fmt.Errorf("remove draft: %w", err)
	}

	return s.Reload()
}

// RejectDraft deletes a draft file.
func (s *Store) RejectDraft(doc *Document) error {
	return os.Remove(doc.FilePath)
}