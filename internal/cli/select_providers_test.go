package cli

import (
	"reflect"
	"testing"
)

func TestSplitProviderList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty", "", nil},
		{"single", "weebcentral", []string{"weebcentral"}},
		{"several", "weebcentral,webtoons", []string{"weebcentral", "webtoons"}},
		{"whitespace and blanks", " weebcentral ,, webtoons ,", []string{"weebcentral", "webtoons"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitProviderList(tt.raw); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("splitProviderList(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}
