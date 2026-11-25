package runner

import (
	"testing"
)

func TestIsInteractiveSession(t *testing.T) {
	tests := []struct {
		args        []string
		interactive bool
	}{
		{[]string{}, true},
		{[]string{"--conversation=0fa98ade-3060-4546-ada4-0652329fdfb6"}, true},
		{[]string{"-r", "0fa98ade-3060-4546-ada4-0652329fdfb6"}, true},
		{[]string{"--help"}, false},
		{[]string{"-h"}, false},
		{[]string{"update"}, false},
		{[]string{"models"}, false},
		{[]string{"help"}, false},
		{[]string{"--print", "hello"}, false},
		{[]string{"-p", "hello"}, false},
	}

	for _, tt := range tests {
		got := IsInteractiveSession(tt.args)
		if got != tt.interactive {
			t.Errorf("IsInteractiveSession(%v) = %v; want %v", tt.args, got, tt.interactive)
		}
	}
}
