package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"herdworkbench/internal/companion"
	"herdworkbench/internal/quality"
)

func TestCompanionTrustProfilesAndRepairBinding(t *testing.T) {
	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)
	b.companionStore = companion.NewStore(t.TempDir())
	root := t.TempDir()
	p := Project{ID: "probe", Name: "Probe", Path: root, PHPBinary: "missing-php.exe"}
	b.projects = []Project{p}
	if _, err := b.Companion(p.ID, "profile-load", nil); err == nil {
		t.Fatal("untrusted companion access")
	}
	b.trusted[p.ID] = true
	if _, err := b.Companion(p.ID, "profile-save", map[string]any{"name": "Daily", "vite": false, "queue": false, "services": []string{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Companion(p.ID, "profile-start", map[string]any{"name": "Daily"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Companion(p.ID, "profile-start", map[string]any{"name": "Daily"}); err == nil {
		t.Fatal("duplicate profile allowed")
	}
	if _, err := b.Companion(p.ID, "profile-stop", nil); err != nil {
		t.Fatal(err)
	}
	plan, err := b.previewRepair(p, "config-clear")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "composer.json"), []byte(`{"changed":true}`), 0600)
	if _, err = b.Companion(p.ID, "repair-apply", map[string]any{"plan": plan.ID, "confirmation": p.Name}); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("stale repair executed: %v", err)
	}
	if _, err = b.Companion(p.ID, "execute-anything", nil); err == nil {
		t.Fatal("unknown operation allowed")
	}
}
func TestRouteDraftEscapingAndNoOverwrite(t *testing.T) {
	draft, err := generateTest("GET", "/users/o'hara", "pest", "200", false)
	if err != nil || !strings.Contains(draft, "o\\'hara") {
		t.Fatalf("unsafe test draft: %s %v", draft, err)
	}
	if _, err = generateTest("GET", "//external.test", "pest", "200", false); err == nil {
		t.Fatal("external test URI allowed")
	}
	authDraftPest, err := generateTest("GET", "/dashboard", "pest", "200", true)
	if err != nil || !strings.Contains(authDraftPest, "$this->actingAs($user)") || !strings.Contains(authDraftPest, "\\App\\Models\\User::factory()->create()") {
		t.Fatalf("pest auth draft missing actingAs: %s %v", authDraftPest, err)
	}
	authDraftPHPUnit, err := generateTest("GET", "/dashboard", "phpunit", "200", true)
	if err != nil || !strings.Contains(authDraftPHPUnit, "$this->actingAs($user)") || !strings.Contains(authDraftPHPUnit, "\\App\\Models\\User::factory()->create()") {
		t.Fatalf("phpunit auth draft missing actingAs: %s %v", authDraftPHPUnit, err)
	}
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "tests", "Feature"), 0700)
	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)
	b.companionStore = companion.NewStore(t.TempDir())
	b.projects = []Project{{ID: "testfile", Name: "Test file", Path: root, PHPBinary: "not-executed.php"}}
	b.trusted["testfile"] = true
	input := map[string]any{"method": "GET", "uri": "/probe", "framework": "pest", "status": "200", "name": "PreservedTest.php", "content": "<?php // original test"}
	if _, err = b.Companion("testfile", "test-save", input); err != nil {
		t.Fatal(err)
	}
	input["content"] = "<?php // replacement"
	if _, err = b.Companion("testfile", "test-save", input); err == nil {
		t.Fatal("existing test overwritten")
	}
	raw, _ := os.ReadFile(filepath.Join(root, "tests", "Feature", "PreservedTest.php"))
	if string(raw) != "<?php // original test" {
		t.Fatal("existing test changed")
	}
}

func TestRequestDraftUsesExampleAndRedactsCredentials(t *testing.T) {
	draft, err := generateRequestTest(map[string]any{"method": "POST", "uri": "/projects", "framework": "pest", "status": "201", "body": `{"name":"example","password":"never-copy-this"}`, "headers": `{"Accept":"application/json","Authorization":"Bearer never-copy-this"}`, "authenticated": true})
	if err != nil || strings.Contains(draft, "never-copy-this") || !strings.Contains(draft, "example") || !strings.Contains(draft, "application/json") || !strings.Contains(draft, "$this->json('POST'") || !strings.Contains(draft, "$this->actingAs($user)") {
		t.Fatalf("incorrect draft: %s %v", draft, err)
	}
}

func TestHistoryResolutionRequiresMatchingCompletedCoverage(t *testing.T) {
	before := quality.AuditReport{ComparisonKey: "config", Findings: []quality.CodeBestPracticeFinding{{Fingerprint: "finding", Rule: "MultiWriteTransactionRequired", Tool: "Workbench heuristic"}}, Analyzers: []quality.AnalyzerResult{{ID: "builtin", Status: "completed", Coverage: "all PHP"}}}
	after := quality.AuditReport{ComparisonKey: "config", Analyzers: []quality.AnalyzerResult{{ID: "builtin", Status: "completed", Coverage: "all PHP"}}}
	if c := comparableChanges(before, after); len(c) != 1 || c[0].Status != "resolved" {
		t.Fatalf("comparable scan: %+v", c)
	}
	after.Analyzers[0].Status = "failed"
	if c := comparableChanges(before, after); c[0].Status != "not_observed" {
		t.Fatal("failed analyzer claimed resolution")
	}
	after.Analyzers[0].Status = "completed"
	after.ComparisonKey = "changed"
	if c := comparableChanges(before, after); c[0].Status != "not_observed" {
		t.Fatal("changed configuration claimed resolution")
	}
}

func TestCompanionOwnedProfileProcessLifecycle(t *testing.T) {
	if os.Getenv("WORKBENCH_RUNNER") == "" {
		t.Skip("set WORKBENCH_RUNNER to exercise the Windows process helper")
	}
	if _, err := exec.LookPath("node.exe"); err != nil {
		t.Skip("Node required")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "node_modules", ".bin"), 0700)
	os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"dev":"vite"}}`), 0600)
	os.WriteFile(filepath.Join(root, "node_modules", ".bin", "vite.cmd"), []byte("@echo off\r\nnode \"%~dp0..\\..\\fake-vite.cjs\" %*\r\n"), 0600)
	os.WriteFile(filepath.Join(root, "fake-vite.cjs"), []byte(`const http=require('node:http');const port=Number(process.argv[process.argv.indexOf('--port')+1]);const server=http.createServer((q,r)=>r.end('owned-profile'));server.listen(port,'127.0.0.1');process.on('SIGBREAK',()=>server.close(()=>process.exit(0)));process.on('SIGINT',()=>server.close(()=>process.exit(0)));`), 0600)
	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)
	b.companionStore = companion.NewStore(t.TempDir())
	php, _ := exec.LookPath("php.exe")
	b.projects = []Project{{ID: "profile", Name: "Profile", Path: root, PHPBinary: php}}
	b.trusted["profile"] = true
	if _, err = b.Companion("profile", "profile-save", map[string]any{"name": "Owned", "vite": true, "vitePort": port}); err != nil {
		t.Fatal(err)
	}
	if _, err = b.Companion("profile", "profile-start", map[string]any{"name": "Owned"}); err != nil {
		t.Fatal(err)
	}
	client := http.Client{Timeout: time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		response, e := client.Get(url)
		if e == nil {
			response.Body.Close()
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("profile service never became reachable")
	}
	if _, err = b.Companion("profile", "profile-stop", nil); err != nil {
		t.Fatal(err)
	}
	check, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("profile left port occupied: %v", err)
	}
	defer check.Close()
	if _, err = b.Companion("profile", "profile-start", map[string]any{"name": "Owned"}); err == nil {
		t.Fatal("occupied port was not detected")
	}
}

func TestCompanionHealthAgainstDiscoveredHerdProjects(t *testing.T) {
	if os.Getenv("WORKBENCH_COMPANION_HERD") != "1" {
		t.Skip("opt in to read-only Herd discovery")
	}
	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)
	if err := b.Discover(); err != nil {
		t.Fatal(err)
	}
	rows := b.WorkspaceHealth()
	if len(rows) == 0 {
		t.Fatal("No discovered projects")
	}
	for _, row := range rows {
		if row.Trusted {
			t.Fatal("discovery unexpectedly trusted project")
		}
		if row.Coverage != "Not run" {
			t.Fatal("fabricated audit coverage")
		}
	}
	t.Logf("Read-only readiness produced %d discovered project rows", len(rows))
}

// Opt-in actual Laravel bridge acceptance. Source framework dependencies are
// read-only; all application files, schema, stores and snapshots are disposable.
func TestCompanionRealLaravelSQLite(t *testing.T) {
	fixture := os.Getenv("WORKBENCH_COMPANION_FIXTURE")
	if fixture == "" {
		t.Skip("set WORKBENCH_COMPANION_FIXTURE to a disposable Laravel skeleton")
	}
	php, err := exec.LookPath("php.exe")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, name := range []string{"bootstrap/cache", "storage/framework/cache/data", "storage/framework/sessions", "storage/framework/views", "storage/logs", "routes", "database/migrations", "tests/Feature", "vendor", "app/Providers"} {
		if err = os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	copyFile := func(rel string) {
		raw, e := os.ReadFile(filepath.Join(fixture, rel))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(root, rel), raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	copyFile("bootstrap/app.php")
	copyFile("artisan")
	copyFile("composer.json")
	escaped := strings.ReplaceAll(filepath.ToSlash(filepath.Join(fixture, "vendor", "autoload.php")), "'", "\\'")
	os.WriteFile(filepath.Join(root, "vendor", "autoload.php"), []byte("<?php require '"+escaped+"';"), 0600)
	os.WriteFile(filepath.Join(root, "bootstrap", "providers.php"), []byte("<?php return [];"), 0600)
	os.WriteFile(filepath.Join(root, "routes", "web.php"), []byte("<?php Illuminate\\Support\\Facades\\Route::get('/probe/{id}',fn()=> 'ok')->middleware('web');"), 0600)
	os.WriteFile(filepath.Join(root, "routes", "console.php"), []byte("<?php"), 0600)
	db := filepath.ToSlash(filepath.Join(root, "database", "database.sqlite"))
	os.WriteFile(db, []byte{}, 0600)
	os.WriteFile(filepath.Join(root, ".env"), []byte("APP_ENV=local\nAPP_KEY=base64:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\nDB_CONNECTION=sqlite\nDB_DATABASE="+db+"\nCACHE_STORE=file\nSESSION_DRIVER=file\nLOG_CHANNEL=single\n"), 0600)
	// Copy only configuration (not the fixture's data, env or service providers).
	if err = filepath.WalkDir(filepath.Join(fixture, "config"), func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(fixture, path)
		dest := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		return os.WriteFile(dest, raw, 0600)
	}); err != nil {
		t.Fatal(err)
	}
	migration := `<?php return new class extends Illuminate\Database\Migrations\Migration { public function up():void { Illuminate\Support\Facades\Schema::create('probes',function($t){$t->id();$t->string('name');}); } public function down():void{Illuminate\Support\Facades\Schema::dropIfExists('probes');} };`
	os.WriteFile(filepath.Join(root, "database/migrations/2026_01_01_000000_create_probes.php"), []byte(migration), 0600)
	b := NewBackend(func() {})
	b.companionStore = companion.NewStore(t.TempDir())
	b.projects = []Project{{ID: "live", Name: "Live", Path: root, PHPBinary: php}}
	b.trusted["live"] = true
	t.Cleanup(b.Shutdown)
	call := func(op string, input map[string]any) map[string]any {
		t.Helper()
		value, e := b.Companion("live", op, input)
		if e != nil {
			t.Fatalf("%s: %v", op, e)
		}
		raw, _ := json.Marshal(value)
		var out map[string]any
		json.Unmarshal(raw, &out)
		return out
	}
	runtime := call("runtime", nil)
	if runtime["environment"] != "local" {
		t.Fatal(runtime)
	}
	migrations := call("migrations", nil)
	if len(migrations["migrations"].([]any)) != 1 {
		t.Fatal(migrations)
	}
	target := "sqlite / " + db
	if _, e := b.Companion("live", "migrate", map[string]any{"confirmation": "wrong"}); e == nil {
		t.Fatal("unconfirmed migration executed")
	}
	preview := call("migration-preview", nil)
	if !strings.Contains(strings.ToLower(fmt.Sprint(preview["output"])), "create table") {
		t.Fatal(preview)
	}
	call("migrate", map[string]any{"confirmation": target})
	call("schema-pin", nil)
	diff := call("schema-diff", nil)
	if len(diff["changes"].([]any)) != 0 {
		t.Fatal(diff)
	}
	routes := call("routes", nil)
	if len(routes["routes"].([]any)) == 0 {
		t.Fatal(routes)
	}
	// Seed cache and session through a disposable app process, never a user project.
	seed := `<?php require __DIR__.'/vendor/autoload.php';$app=require __DIR__.'/bootstrap/app.php';$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();Illuminate\Support\Facades\Cache::put('probe',['answer'=>42],600);file_put_contents(storage_path('framework/sessions/session-probe'),'probe-session-data');$app['db']->table('probes')->insert(['name'=>'original']);`
	os.WriteFile(filepath.Join(root, "seed.php"), []byte(seed), 0600)
	command := exec.Command(php, filepath.Join(root, "seed.php"))
	command.Dir = root
	if out, e := command.CombinedOutput(); e != nil {
		t.Fatalf("seed %v %s", e, out)
	}
	cache := call("cache", nil)
	entries := cache["entries"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["value"] != "[masked]" {
		t.Fatal(cache)
	}
	key := entries[0].(map[string]any)["key"].(string)
	revealed := call("cache", map[string]any{"key": key, "reveal": true})
	if !strings.Contains(fmt.Sprint(revealed), "42") {
		t.Fatal(revealed)
	}
	if _, e := b.Companion("live", "cache-delete", map[string]any{"key": key}); e == nil {
		t.Fatal("unconfirmed cache deletion")
	}
	call("cache-delete", map[string]any{"key": key, "confirmation": key})
	if len(call("cache", nil)["entries"].([]any)) != 0 {
		t.Fatal("cache entry remains")
	}
	sessions := call("sessions", nil)
	if len(sessions["entries"].([]any)) != 1 {
		t.Fatal(sessions)
	}
	call("session-delete", map[string]any{"key": "session-probe", "confirmation": "session-probe"})
	if len(call("sessions", nil)["entries"].([]any)) != 0 {
		t.Fatal("session remains")
	}
	snapshot := call("snapshot-create", nil)
	sid := snapshot["id"].(string)
	call("rollback", map[string]any{"confirmation": target})
	if _, e := b.Companion("live", "snapshot-restore", map[string]any{"snapshot": sid, "confirmation": "wrong"}); e == nil {
		t.Fatal("unconfirmed restore")
	}
	call("snapshot-restore", map[string]any{"snapshot": sid, "confirmation": target})
	restored := call("migrations", nil)
	if restored["migrations"].([]any)[0].(map[string]any)["status"] != "applied" {
		t.Fatal("restore did not recover schema and migration history")
	}
	var all []SnapshotManifest
	all, _ = b.listSnapshots("live")
	if len(all) != 2 {
		t.Fatalf("recovery snapshot missing: %+v", all)
	}
	// Database cache prefixes often contain underscores. Verify isolation and
	// literal search behavior against SQLite rather than relying on LIKE defaults.
	os.WriteFile(filepath.Join(root, ".env"), []byte("APP_ENV=local\nAPP_KEY=base64:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\nDB_CONNECTION=sqlite\nDB_DATABASE="+db+"\nCACHE_STORE=database\nCACHE_PREFIX=probe_prefix_\nSESSION_DRIVER=database\nLOG_CHANNEL=single\n"), 0600)
	databaseSeed := `<?php require __DIR__.'/vendor/autoload.php';$app=require __DIR__.'/bootstrap/app.php';$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();$schema=$app['db']->connection()->getSchemaBuilder();$schema->create('cache',function($t){$t->string('key')->primary();$t->text('value');$t->integer('expiration');});$schema->create('sessions',function($t){$t->string('id')->primary();$t->integer('last_activity');$t->text('payload');});Illuminate\Support\Facades\Cache::put('database_probe','db-value',600);$app['db']->table('cache')->insert(['key'=>'another_app_database_probe','value'=>'outside','expiration'=>time()+600]);$app['db']->table('sessions')->insert(['id'=>'database-session','last_activity'=>time(),'payload'=>base64_encode('safe-db-session')]);`
	os.WriteFile(filepath.Join(root, "seed.php"), []byte(databaseSeed), 0600)
	command = exec.Command(php, filepath.Join(root, "seed.php"))
	command.Dir = root
	if output, e := command.CombinedOutput(); e != nil {
		t.Fatalf("database store seed: %v %s", e, output)
	}
	databaseCache := call("cache", map[string]any{"search": "database_probe"})
	databaseEntries := databaseCache["entries"].([]any)
	if len(databaseEntries) != 1 || databaseEntries[0].(map[string]any)["key"] != "database_probe" {
		t.Fatalf("prefix isolation failed: %+v", databaseCache)
	}
	call("cache-delete", map[string]any{"key": "database_probe", "confirmation": "database_probe"})
	if len(call("cache", nil)["entries"].([]any)) != 0 {
		t.Fatal("database cache deletion failed")
	}
	if len(call("sessions", nil)["entries"].([]any)) != 1 {
		t.Fatal("database session missing")
	}
	call("session-delete", map[string]any{"key": "database-session", "confirmation": "database-session"})
	if len(call("sessions", nil)["entries"].([]any)) != 0 {
		t.Fatal("database session deletion failed")
	}
	// A migration's explicit connection must not bypass target confirmation.
	altPath := filepath.ToSlash(filepath.Join(root, "database", "alternate.sqlite"))
	os.WriteFile(altPath, []byte{}, 0600)
	configSource := strings.ReplaceAll(filepath.ToSlash(filepath.Join(fixture, "config", "database.php")), "'", "\\'")
	configPHP := fmt.Sprintf("<?php $config=require '%s';$config['connections']['alternate']=['driver'=>'sqlite','database'=>'%s','prefix'=>''];return $config;", configSource, altPath)
	os.WriteFile(filepath.Join(root, "config", "database.php"), []byte(configPHP), 0600)
	otherMigration := `<?php return new class extends Illuminate\Database\Migrations\Migration { public function up():void{Illuminate\Support\Facades\Schema::connection('alternate')->create('should_not_exist',function($t){$t->id();});} public function down():void{} };`
	os.WriteFile(filepath.Join(root, "database/migrations/2026_01_02_000000_other_connection.php"), []byte(otherMigration), 0600)
	if _, e := b.Companion("live", "migrate", map[string]any{"confirmation": target}); e == nil {
		t.Fatal("migration crossed the confirmed connection boundary")
	}
	info, e := os.Stat(altPath)
	if e != nil || info.Size() != 0 {
		t.Fatal("unconfirmed alternate database was modified")
	}
	// Target mismatch must be rejected before creating a recovery artifact.
	os.WriteFile(filepath.Join(root, ".env"), []byte("APP_ENV=local\nDB_CONNECTION=sqlite\nDB_DATABASE="+filepath.ToSlash(filepath.Join(root, "other.sqlite"))+"\n"), 0600)
	if _, e := b.Companion("live", "snapshot-restore", map[string]any{"snapshot": sid, "confirmation": target}); e == nil {
		t.Fatal("cross-database restore accepted")
	}
	// Runtime safety is independent of frontend controls.
	os.WriteFile(filepath.Join(root, ".env"), []byte("APP_ENV=production\n"), 0600)
	if _, e := companion.Run(context.Background(), php, root, "runtime", nil, time.Minute); e == nil {
		t.Fatal("production environment accepted")
	}
}
