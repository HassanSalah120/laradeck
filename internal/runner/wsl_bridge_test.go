package runner

import (
	"strings"
	"testing"
)

func TestWindowsToWSLPath(t *testing.T) {
	cases := []struct {
		winPath  string
		expected string
		wantErr  bool
	}{
		{`C:\Users\test\project`, "/mnt/c/Users/test/project", false},
		{`G:\laradeck\outputs\herd-workbench`, "/mnt/g/laradeck/outputs/herd-workbench", false},
		{`relative\path`, "", true},
		{`invalid`, "", true},
	}

	for _, c := range cases {
		out, err := WindowsToWSLPath(c.winPath)
		if c.wantErr {
			if err == nil {
				t.Errorf("expected error for path %s, got none", c.winPath)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for %s: %v", c.winPath, err)
			}
			if out != c.expected {
				t.Errorf("expected %s, got %s", c.expected, out)
			}
		}
	}
}

func TestBuildHorizonWSLCommand(t *testing.T) {
	args, err := BuildHorizonWSLCommand("Ubuntu", `G:\laradeck\project`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-d Ubuntu") {
		t.Errorf("missing distro argument: %s", joined)
	}
	if !strings.Contains(joined, "--cd /mnt/g/laradeck/project") {
		t.Errorf("missing converted path argument: %s", joined)
	}
	if !strings.Contains(joined, "php artisan horizon") {
		t.Errorf("missing horizon command: %s", joined)
	}
}
