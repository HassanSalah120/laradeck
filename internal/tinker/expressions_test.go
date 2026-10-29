package tinker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestTinkerReturnsTrailingExpressionsWithoutRepeatingExecution(t *testing.T) {
	php, err := exec.LookPath("php")
	if err != nil {
		t.Skip("PHP required")
	}
	helper := filepath.Join(t.TempDir(), "runner.exe")
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go.exe"), "build", "-o", helper, "../../cmd/runner")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	t.Setenv("WORKBENCH_RUNNER", helper)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "vendor"), 0700)
	os.MkdirAll(filepath.Join(root, "bootstrap"), 0700)
	os.WriteFile(filepath.Join(root, "vendor", "autoload.php"), []byte("<?php\n"), 0600)
	os.WriteFile(filepath.Join(root, "bootstrap", "app.php"), []byte("<?php return new stdClass;"), 0600)
	cases := []struct {
		code, want string
		hasError   bool
	}{
		{"1 + 1;", "=> 2", false}, {"1 + 1", "=> 2", false}, {"return 1 + 1;", "=> 2", false},
		{"$x = 2; $x + 3;", "=> 5", false}, {`'a;b';`, "=> 'a;b'", false},
		{"1 + 1; // trailing comment", "=> 2", false}, {"echo 'hello';", "hello", false},
		{`for ($i=0; $i<2; $i++) { echo $i; }`, "01", false},
		{`(function(){ echo 'once'; throw new ParseError('fixture'); })();`, "once", true},
	}
	svc := NewTinkerService()
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			res, err := svc.EvaluateContext(context.Background(), php, root, tc.code, 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(res.Output) != tc.want || (res.Error != "") != tc.hasError {
				t.Fatalf("result %+v expected %q", res, tc.want)
			}
		})
	}
}
