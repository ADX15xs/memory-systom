// Package tag provides tag normalization, alias mapping, and multi-dimension validation.
package tag

import (
	"fmt"
	"strings"
	"sync"
)

// Manager handles tag operations.
type Manager struct {
	mu      sync.RWMutex
	aliases map[string][]string // canonical → variants
	reverse map[string]string   // variant → canonical
}

// NewManager creates a tag manager from an alias map (canonical → [variants...]).
func NewManager(aliases map[string][]string) *Manager {
	m := &Manager{
		aliases: aliases,
		reverse: make(map[string]string),
	}
	for canonical, variants := range aliases {
		m.reverse[canonical] = canonical
		for _, v := range variants {
			m.reverse[v] = canonical
		}
	}
	return m
}

// Normalize normalizes a tag: lowercase, spaces→hyphens, remove underscores.
func Normalize(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	tag = strings.ReplaceAll(tag, "_", "-")
	tag = strings.ReplaceAll(tag, " ", "-")
	tag = strings.ReplaceAll(tag, "\t", "-")
	// Collapse multiple hyphens
	for strings.Contains(tag, "--") {
		tag = strings.ReplaceAll(tag, "--", "-")
	}
	return tag
}

// Resolve returns the canonical form of a tag (applying aliases then normalizing).
func (m *Manager) Resolve(tag string) string {
	normalized := Normalize(tag)
	m.mu.RLock()
	defer m.mu.RUnlock()
	if canonical, ok := m.reverse[normalized]; ok {
		return canonical
	}
	return normalized
}

// ResolveAll resolves and normalizes a slice of tags.
func (m *Manager) ResolveAll(tags []string) []string {
	seen := make(map[string]bool, len(tags))
	resolved := make([]string, 0, len(tags))
	for _, t := range tags {
		r := m.Resolve(t)
		if !seen[r] {
			seen[r] = true
			resolved = append(resolved, r)
		}
	}
	return resolved
}

// dimension prefixes
const (
	DimScene  = "场景"
	DimDomain = "域"
	DimType   = "类型"
	DimClient = "甲方"
)

// ValidateResult describes the outcome of a tag validation.
type ValidateResult struct {
	Valid      bool
	Dimensions map[string]int // dimension → count of tags in that dimension
	Errors     []string
}

// Validate checks that tags cover at least 3 of the 4 required dimensions
// and that each tag is properly formatted.
func Validate(tags []string) ValidateResult {
	r := ValidateResult{
		Valid:      true,
		Dimensions: map[string]int{DimScene: 0, DimDomain: 0, DimType: 0, DimClient: 0},
		Errors:     make([]string, 0),
	}

	for _, t := range tags {
		// Check format
		if t == "" {
			r.Errors = append(r.Errors, "tag must not be empty")
			r.Valid = false
			continue
		}

		// Check for forbidden characters
		if strings.ContainsAny(t, " _\t") {
			r.Errors = append(r.Errors, fmt.Sprintf("tag %q contains spaces, underscores, or tabs", t))
			r.Valid = false
		}

		// Detect dimension
		parts := strings.SplitN(t, ":", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0]+":", DimScene+":") {
			r.Dimensions[DimScene]++
		} else if len(parts) == 2 && strings.HasPrefix(parts[0]+":", DimDomain+":") {
			r.Dimensions[DimDomain]++
		} else if len(parts) == 2 && strings.HasPrefix(parts[0]+":", DimType+":") {
			r.Dimensions[DimType]++
		} else if len(parts) == 2 && strings.HasPrefix(parts[0]+":", DimClient+":") {
			r.Dimensions[DimClient]++
		}
	}

	covered := 0
	for _, count := range r.Dimensions {
		if count > 0 {
			covered++
		}
	}

	if covered < 3 {
		r.Errors = append(r.Errors,
			fmt.Sprintf("tags must cover at least 3 of 4 dimensions (场景/域/类型/甲方), got %d", covered))
		r.Valid = false
	}

	return r
}
