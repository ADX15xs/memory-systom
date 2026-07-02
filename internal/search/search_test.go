package search

import (
	"testing"

	"knowledge-base/internal/fs"
	"knowledge-base/internal/tag"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		input string
		min   int // at least this many terms
	}{
		{"go k8s 部署", 3},
		{"GoServiceDeploy", 1}, // at minimum the full lowercased term is found
		{"", 0},
		{"the a an", 0}, // stop words only
	}
	for _, tt := range tests {
		got := tokenize(tt.input)
		if len(got) < tt.min {
			t.Errorf("tokenize(%q) = %v (len=%d), want at least %d terms", tt.input, got, len(got), tt.min)
		}
	}
}

func TestBuildIndexAndSearch(t *testing.T) {
	mgr := tag.NewManager(map[string][]string{
		"go": {"golang"},
		"k8s": {"kubernetes"},
	})
	engine := New(mgr)

	docs := []*fs.Document{
		{
			Title:   "Go 代码规范",
			Content: "包名小写，使用gofmt，错误检查",
			Tags:    []string{"场景:开发", "域:Go", "类型:规范", "甲方:通用"},
		},
		{
			Title:   "K8s 部署踩坑",
			Content: "Pod启动缓慢，OOMKilled，滚动更新失败",
			Tags:    []string{"场景:部署", "域:K8s", "类型:避坑", "甲方:XX银行"},
		},
		{
			Title:   "Docker 镜像优化",
			Content: "多阶段构建，减少镜像层数",
			Tags:    []string{"场景:部署", "域:Docker", "类型:指南", "甲方:通用"},
		},
	}

	engine.BuildIndex(docs)

	t.Run("search by tag", func(t *testing.T) {
		results := engine.Search("go")
		if len(results) == 0 {
			t.Fatal("expected results for 'go'")
		}
		if results[0].Document.Title != "Go 代码规范" {
			t.Errorf("top result should be 'Go 代码规范', got %q", results[0].Document.Title)
		}
	})

	t.Run("search by alias", func(t *testing.T) {
		results := engine.Search("golang")
		if len(results) == 0 {
			t.Fatal("expected results for 'golang' (alias)")
		}
		if results[0].Document.Title != "Go 代码规范" {
			t.Errorf("top result should be 'Go 代码规范', got %q", results[0].Document.Title)
		}
	})

	t.Run("search by content", func(t *testing.T) {
		results := engine.Search("oomkilled")
		if len(results) == 0 {
			t.Fatal("expected results for 'oomkilled'")
		}
		if results[0].Document.Title != "K8s 部署踩坑" {
			t.Errorf("top result should be 'K8s 部署踩坑', got %q", results[0].Document.Title)
		}
	})

	t.Run("search with multiple terms", func(t *testing.T) {
		results := engine.Search("k8s 部署")
		if len(results) == 0 {
			t.Fatal("expected results for 'k8s 部署'")
		}
	})

	t.Run("search no results", func(t *testing.T) {
		results := engine.Search("zzzzz_notexist")
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("downgrade strategy", func(t *testing.T) {
		// Results should still be found even with stop words
		results := engine.Search("the go 规范")
		if len(results) == 0 {
			t.Errorf("expected results after stop word removal")
		}
	})

	t.Run("top 5 limit", func(t *testing.T) {
		// Add many docs to test limit
		bigDocs := make([]*fs.Document, 10)
		for i := 0; i < 10; i++ {
			bigDocs[i] = &fs.Document{
				Title:   "Doc",
				Content: "searchable content for testing",
				Tags:    []string{"场景:开发", "域:Go", "类型:规范", "甲方:通用"},
			}
		}
		engine.BuildIndex(append(docs, bigDocs...))
		results := engine.Search("searchable")
		if len(results) > 5 {
			t.Errorf("expected at most 5 results, got %d", len(results))
		}
	})
}