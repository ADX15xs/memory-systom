package tag

import (
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Go", "go"},
		{"Go-Modules", "go-modules"},
		{"go lang", "go-lang"},
		{"  Docker  ", "docker"},
		{"K8s_Deploy", "k8s-deploy"},
		{"My__Tag", "my-tag"},
	}
	for _, tt := range tests {
		got := Normalize(tt.input)
		if got != tt.expected {
			t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestResolve(t *testing.T) {
	m := NewManager(map[string][]string{
		"go": {"golang", "go语言"},
	})
	tests := []struct {
		input    string
		expected string
	}{
		{"golang", "go"},
		{"Go语言", "go"},
		{"Go", "go"},
		{"rust", "rust"},
	}
	for _, tt := range tests {
		got := m.Resolve(tt.input)
		if got != tt.expected {
			t.Errorf("Resolve(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name  string
		tags  []string
		valid bool
	}{
		{
			name:  "valid all 4 dimensions",
			tags:  []string{"场景:部署", "域:Go", "类型:指南", "甲方:XX银行"},
			valid: true,
		},
		{
			name:  "valid 3 dimensions",
			tags:  []string{"场景:部署", "域:Go", "类型:指南"},
			valid: true,
		},
		{
			name:  "invalid only 2 dimensions",
			tags:  []string{"场景:部署", "域:Go"},
			valid: false,
		},
		{
			name:  "forbidden characters",
			tags:  []string{"场景:部署", "域:Go", "类型:good practice", "甲方:XX银行"},
			valid: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Validate(tt.tags)
			if r.Valid != tt.valid {
				t.Errorf("Validate(%v).Valid = %v, want %v; errors: %v", tt.tags, r.Valid, tt.valid, r.Errors)
			}
		})
	}
}
