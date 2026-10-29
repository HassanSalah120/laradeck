package quality

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBatchPreservesProjectContextAcrossThreshold(t *testing.T) {
	php, err := exec.LookPath("php")
	if err != nil {
		t.Skip("PHP is required")
	}
	root := t.TempDir()
	write := func(rel, code string) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0700)
		if err := os.WriteFile(p, []byte(code), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("app/Http/Controllers/ReviewController.php", `<?php namespace App\Http\Controllers; class ReviewController { public function store(){ User::create(['x'=>1]); Profile::create(['x'=>2]); } }`)
	write("app/Http/Middleware/AtomicMiddleware.php", `<?php namespace App\Http\Middleware; class AtomicMiddleware { public function handle($request,$next){ return DB::transaction(function() use ($request,$next){ return $next($request); }); } }`)
	write("routes/web.php", `<?php Route::middleware([AtomicMiddleware::class])->group(function(){ Route::post('/review', [ReviewController::class,'store']); });`)
	for i := 0; i < 97; i++ {
		write(fmt.Sprintf("app/Models/Filler%03d.php", i), "<?php // inert filler\n")
	}
	count := func() int {
		files, err := collectPHPFiles(root)
		if err != nil {
			t.Fatal(err)
		}
		f, err := runPHPBatchAnalyzer(context.Background(), php, files, ProjectContext{ProjectPath: root})
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, v := range f {
			if v.Rule == "MultiWriteTransactionRequired" {
				n++
			}
		}
		return n
	}
	before := count()
	write("app/Models/Filler098.php", "<?php // inert filler\n")
	after := count()
	t.Logf("Same transactional controller: 100 source files => %d warnings; 101 source files => %d warnings", before, after)
	if before != 0 || after != 0 {
		t.Fatalf("cross-file transaction context lost: before=%d after=%d", before, after)
	}
}
