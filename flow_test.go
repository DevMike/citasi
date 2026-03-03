package main

import (
	"testing"
)

func TestContainsNoAvailability(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "exact match",
			text:     "En este momento no hay citas disponibles",
			expected: true,
		},
		{
			name:     "embedded in larger text",
			text:     "Lo sentimos, En este momento no hay citas disponibles. Intente más tarde.",
			expected: true,
		},
		{
			name:     "no match - availability exists",
			text:     "Seleccione una fecha para su cita",
			expected: false,
		},
		{
			name:     "empty string",
			text:     "",
			expected: false,
		},
		{
			name:     "partial match should not trigger",
			text:     "En este momento no hay",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsNoAvailability(tt.text)
			if got != tt.expected {
				t.Errorf("containsNoAvailability(%q) = %v, want %v", tt.text, got, tt.expected)
			}
		})
	}
}

func TestGetBaseURL(t *testing.T) {
	tests := []struct {
		province string
		contains string
	}{
		{"8", "icpplustieb"},
		{"28", "icpplustiem"},
		{"1", "icpplus/citar"},
		{"33", "icpplus/citar"},
	}

	for _, tt := range tests {
		t.Run("province_"+tt.province, func(t *testing.T) {
			url := getBaseURL(tt.province)
			if !containsSubstring(url, tt.contains) {
				t.Errorf("getBaseURL(%q) = %q, expected to contain %q", tt.province, url, tt.contains)
			}
			if !containsSubstring(url, "p="+tt.province) {
				t.Errorf("getBaseURL(%q) = %q, expected to contain p=%s", tt.province, url, tt.province)
			}
		})
	}
}

func TestDocTypeSelector(t *testing.T) {
	tests := []struct {
		docType  string
		expected string
	}{
		{"nie", SelDocNIE},
		{"passport", SelDocPassport},
		{"dni", SelDocDNI},
		{"unknown", SelDocNIE}, // defaults to NIE
	}

	for _, tt := range tests {
		t.Run(tt.docType, func(t *testing.T) {
			got := docTypeSelector(tt.docType)
			if got != tt.expected {
				t.Errorf("docTypeSelector(%q) = %q, want %q", tt.docType, got, tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
			}
		})
	}
}

func TestStateTracker(t *testing.T) {
	tracker := NewStateTracker()
	tracker.cooldown = 0 // disable cooldown for testing

	// First unavailable — no notification
	if tracker.ShouldNotify(false) {
		t.Error("should not notify on initial false")
	}

	// First available — should notify
	if !tracker.ShouldNotify(true) {
		t.Error("should notify on first true")
	}

	// Still available — should notify (cooldown=0)
	if !tracker.ShouldNotify(true) {
		t.Error("should notify when still true with 0 cooldown")
	}

	// Back to unavailable
	if tracker.ShouldNotify(false) {
		t.Error("should not notify on false")
	}

	// Available again — should notify (transition)
	if !tracker.ShouldNotify(true) {
		t.Error("should notify on false→true transition")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
