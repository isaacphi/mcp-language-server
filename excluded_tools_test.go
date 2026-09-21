package main

import "testing"

func TestExcludedToolNames(t *testing.T) {
	for _, valid := range []string{"", "definition,references", "edit_file, rename_symbol"} {
		if _, err := excludedToolNames(valid); err != nil {
			t.Fatal(err)
		}
	}
	for _, invalid := range []string{"definitions", "definition,", "*"} {
		if _, err := excludedToolNames(invalid); err == nil {
			t.Fatalf("accepted %q", invalid)
		}
	}
}
