package ui_test

import (
	"bytes"
	"testing"

	"github.com/NiR-/notpecl/ui"
)

func TestInteractiveUI(t *testing.T) {
	testcases := map[string]struct {
		question   string
		defaultVal string
		written    string
		expected   string
	}{
		"with a specified value": {
			question:   "do you want to enable?",
			defaultVal: "no",
			written:    "yes\n",
			expected:   "yes",
		},
		"with the default value": {
			question:   "do you want to enable?",
			defaultVal: "no",
			written:    "\n",
			expected:   "no",
		},
	}

	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			in := &bytes.Buffer{}
			out := &bytes.Buffer{}
			in.Write([]byte(tc.written + "\n"))

			ui := ui.NewInteractiveUI(in, out)
			ret, err := ui.Prompt(tc.question, tc.defaultVal)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if ret != tc.expected {
				t.Fatalf("Expected: %s\nGot: %s", tc.expected, ret)
			}
		})
	}
}

func TestNonInteractiveUI(t *testing.T) {
	ui := ui.NewNonInteractiveUI()
	ret, _ := ui.Prompt("do you want to enable?", "42")
	if ret != "42" {
		t.Fatalf("Expected: 42\nGot: %s", ret)
	}
}
