package domain_test

import (
	"testing"

	"github.com/omarys/labrador/internal/domain"
)

func TestParseChapterNumber(t *testing.T) {
	tests := []struct {
		title    string
		expected *float64
	}{
		{"Chapter 105", ptr(105.0)},
		{"Ch. 50.5", ptr(50.5)},
		{"Season 1 Chapter 40", ptr(40.0)},
		{"[Season 2] Chapter 45 - The Battle", ptr(45.0)},
		{"Chapter 50 [Season 1 Finale]", ptr(50.0)},
		{"Chapter 51 - Season 2 Premiere", ptr(51.0)},
		{"Season 2 Episode 1", ptr(1.0)},
		{"Season 1 Episode 20 [End of Season 1]", ptr(20.0)},
		{"S2 Ch. 5 - Return of the King", ptr(5.0)},
		{"Season 1 - 40 - The End", ptr(40.0)},
		{"Season 2 - 01 - A New Beginning", ptr(1.0)},
		{"45.5 - Bonus Chapter", ptr(45.5)},
		{"[0045] The Battle", ptr(45.0)},
		{"Season 1 Finale - Hiatus Notice", nil},
	}

	for _, tc := range tests {
		got := domain.ParseChapterNumber(tc.title)
		if tc.expected == nil {
			if got != nil {
				t.Errorf("title %q: expected nil, got %v", tc.title, *got)
			}
		} else {
			if got == nil {
				t.Errorf("title %q: expected %v, got nil", tc.title, *tc.expected)
			} else if *got != *tc.expected {
				t.Errorf("title %q: expected %v, got %v", tc.title, *tc.expected, *got)
			}
		}
	}
}

func ptr(f float64) *float64 {
	return &f
}
