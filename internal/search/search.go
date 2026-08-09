// Package search implements an in-memory inverted-index search engine
// for knowledge documents.
package search

import (
	"sort"
	"strings"
	"sync"

	"knowledge-base/internal/fs"
	"knowledge-base/internal/tag"
)

// stopWords are common terms excluded from search to improve recall.
var stopWords = map[string]bool{
	"的": true, "了": true, "是": true, "在": true, "有": true,
	"和": true, "与": true, "不": true, "也": true, "就": true,
	"都": true, "而": true, "及": true, "但": true, "或": true,
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "in": true, "on": true,
	"at": true, "to": true, "for": true, "of": true, "with": true,
	"and": true, "or": true, "not": true, "this": true, "that": true,
}

// Engine is the inverted-index search engine.
type Engine struct {
	mu     sync.RWMutex
	docs   []*fs.Document
	index  map[string][]int // term → document indices
	tagMgr *tag.Manager
}

// New creates an empty search engine.
func New(tagMgr *tag.Manager) *Engine {
	return &Engine{
		index:  make(map[string][]int),
		tagMgr: tagMgr,
	}
}

// BuildIndex constructs the inverted index from a document list.
func (e *Engine) BuildIndex(docs []*fs.Document) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.docs = docs
	e.index = make(map[string][]int)

	for docIdx, doc := range docs {
		// Terms from tags (resolved)
		for _, t := range doc.Tags {
			resolved := e.tagMgr.Resolve(t)
			e.addTerm(resolved, docIdx)
			// Also index each dimension:value part
			parts := strings.SplitN(resolved, ":", 2)
			if len(parts) == 2 {
				e.addTerm(parts[1], docIdx)
			}
		}

		// Terms from title
		titleTerms := tokenize(doc.Title)
		for _, term := range titleTerms {
			e.addTerm(term, docIdx)
		}

		// Terms from content
		contentTerms := tokenize(doc.Content)
		for _, term := range contentTerms {
			e.addTerm(term, docIdx)
		}
	}
}

func (e *Engine) addTerm(term string, docIdx int) {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return
	}
	e.index[term] = append(e.index[term], docIdx)
}

// Result is a single search result.
type Result struct {
	Document *fs.Document
	Score    int // lower is better; 0 = best
}

// IndexKeys returns all keys in the inverted index (for debugging).
func (e *Engine) IndexKeys() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	keys := make([]string, 0, len(e.index))
	for k := range e.index {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Search runs a query against the index and returns ranked results.
func (e *Engine) Search(query string) []Result {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.docs) == 0 {
		return nil
	}

	terms := tokenize(query)
	if len(terms) == 0 {
		return nil
	}

	// Score each document: count how many query terms match
	scores := make(map[int]int) // docIdx → hit count

	for _, term := range terms {
		// Try exact term match first
		matches, ok := e.index[term]
		if ok {
			for _, docIdx := range matches {
				scores[docIdx]++
			}
			continue
		}

		// Try stem/prefix match (contains)
		for indexedTerm, docIndices := range e.index {
			if strings.Contains(indexedTerm, term) || strings.Contains(term, indexedTerm) {
				for _, docIdx := range docIndices {
					scores[docIdx]++
				}
			}
		}
	}

	// Build results sorted by score descending
	results := make([]Result, 0, len(scores))
	for docIdx, hitCount := range scores {
		doc := *e.docs[docIdx]
		results = append(results, Result{Document: &doc, Score: -hitCount})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score < results[j].Score
		}
		return results[i].Document.Title < results[j].Document.Title
	})

	// Apply downgrade strategy if < 3 results: remove stop words and retry
	if len(results) < 3 && len(scores) < 3 {
		// Remove stop words and requery
		filteredTerms := make([]string, 0, len(terms))
		for _, t := range terms {
			if !stopWords[t] {
				filteredTerms = append(filteredTerms, t)
			}
		}

		if len(filteredTerms) < len(terms) {
			scores = make(map[int]int)
			for _, term := range filteredTerms {
				matches, ok := e.index[term]
				if ok {
					for _, docIdx := range matches {
						scores[docIdx]++
					}
				} else {
					for indexedTerm, docIndices := range e.index {
						if strings.Contains(indexedTerm, term) || strings.Contains(term, indexedTerm) {
							for _, docIdx := range docIndices {
								scores[docIdx]++
							}
						}
					}
				}
			}
			results = make([]Result, 0, len(scores))
			for docIdx, hitCount := range scores {
				doc := *e.docs[docIdx]
				results = append(results, Result{Document: &doc, Score: -hitCount})
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].Score != results[j].Score {
					return results[i].Score < results[j].Score
				}
				return results[i].Document.Title < results[j].Document.Title
			})
		}
	}

	// Limit to top 5
	if len(results) > 5 {
		results = results[:5]
	}

	return results
}

// tokenize splits text into searchable terms.
// Supports Chinese by splitting on punctuation and whitespace.
// Supports English camelCase splitting.
func tokenize(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}

	seen := make(map[string]bool)
	terms := make([]string, 0)

	// Split by whitespace and common punctuation
	separators := func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
			r == ',' || r == '，' || r == '。' || r == '.' ||
			r == '、' || r == '；' || r == ';' || r == '：' ||
			r == ':' || r == '？' || r == '?' || r == '！' ||
			r == '!' || r == '（' || r == '(' || r == '）' ||
			r == ')' || r == '"' || r == '「' ||
			r == '」' || r == '{' || r == '}' || r == '/' ||
			r == '\\' || r == '-'
	}

	parts := strings.FieldsFunc(text, separators)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || stopWords[part] {
			continue
		}
		if !seen[part] {
			seen[part] = true
			terms = append(terms, part)
		}

		// CamelCase splitting for English terms
		camelTerms := splitCamelCase(part)
		for _, ct := range camelTerms {
			ct = strings.TrimSpace(ct)
			if ct != "" && ct != part && !stopWords[ct] && !seen[ct] {
				seen[ct] = true
				terms = append(terms, ct)
			}
		}
	}

	return terms
}

// splitCamelCase splits a term on CamelCase boundaries.
func splitCamelCase(s string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	current := make([]rune, 0)
	runes := []rune(s)

	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' && runes[i-1] >= 'a' && runes[i-1] <= 'z' {
			// Transition lowercase → uppercase: split
			if len(current) > 0 {
				parts = append(parts, strings.ToLower(string(current)))
				current = make([]rune, 0)
			}
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		parts = append(parts, strings.ToLower(string(current)))
	}
	return parts
}
