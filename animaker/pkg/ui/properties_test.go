package ui

import (
	"reflect"
	"testing"
)

func TestSheetPickerOptions(t *testing.T) {
	loaded := []string{"body", "hair"}

	tests := []struct {
		name    string
		loaded  []string
		current string
		want    []string
	}{
		{"no current binding", loaded, "", []string{"body", "hair"}},
		{"current is loaded", loaded, "hair", []string{"body", "hair"}},
		// A track opened without its art still has to display its binding,
		// otherwise the Select blanks and looks like the part lost its sheet.
		{"current not loaded", loaded, "sword", []string{"sword", "body", "hair"}},
		{"nothing loaded", nil, "sword", []string{"sword"}},
		{"nothing at all", nil, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sheetPickerOptions(tt.loaded, tt.current)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sheetPickerOptions(%v, %q) = %v, want %v", tt.loaded, tt.current, got, tt.want)
			}
		})
	}
}
