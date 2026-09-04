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

func TestParseChapterNumberFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected *float64
	}{
		{"[0378]_Chapter_9_-_The_Golden_Age.cbz", ptr(9.0)},
		{"[0001]_Chapter_386_-_Can_You_Seize_the_Wandering_Birds_in_the_Clouds.cbz", ptr(386.0)},
		{"[0067]_Chapter_1.cbz", ptr(1.0)},
		{"Berserk - Chapter 1 - The Golden Age.cbz", ptr(1.0)},
		{"Solo Leveling - c105.cbz", ptr(105.0)},
		{"[0045]_Episode_209_Aug_15.cbz", ptr(209.0)},
		{"Chapter 12.5.cbz", ptr(12.5)},
		{"[0045].cbz", ptr(45.0)},
	}

	for _, tc := range tests {
		got := domain.ParseChapterNumberFromFilename(tc.filename)
		if tc.expected == nil {
			if got != nil {
				t.Errorf("filename %q: expected nil, got %v", tc.filename, *got)
			}
		} else {
			if got == nil {
				t.Errorf("filename %q: expected %v, got nil", tc.filename, *tc.expected)
			} else if *got != *tc.expected {
				t.Errorf("filename %q: expected %v, got %v", tc.filename, *tc.expected, *got)
			}
		}
	}
}

func ptr(f float64) *float64 {
	return &f
}
