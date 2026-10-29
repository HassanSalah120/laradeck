package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEloquentTypedUpdates(t *testing.T) {
	autoload := os.Getenv("WORKBENCH_ELOQUENT_AUTOLOAD")
	if autoload == "" {
		t.Skip("set WORKBENCH_ELOQUENT_AUTOLOAD to a disposable fixture's vendor/autoload.php")
	}
	php, err := exec.LookPath("php")
	if err != nil {
		t.Skip("PHP required")
	}
	helper := filepath.Join(t.TempDir(), "runner.exe")
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go.exe"), "build", "-o", helper, "../../cmd/runner")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("helper: %v %s", err, output)
	}
	t.Setenv("WORKBENCH_RUNNER", helper)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "vendor"), 0700)
	os.MkdirAll(filepath.Join(root, "bootstrap"), 0700)
	autoload = strings.ReplaceAll(filepath.ToSlash(autoload), "'", "\\'")
	os.WriteFile(filepath.Join(root, "vendor", "autoload.php"), []byte(fmt.Sprintf("<?php require '%s';", autoload)), 0600)
	bootstrap := `<?php
 $capsule=new Illuminate\Database\Capsule\Manager;
 $capsule->addConnection(['driver'=>'sqlite','database'=>':memory:','prefix'=>'']);
 $capsule->setAsGlobal();$capsule->bootEloquent();
 $capsule->schema()->create('review_probes',function($t){$t->increments('id');$t->boolean('active')->nullable();$t->text('meta')->nullable();});
 class ReviewProbe extends Illuminate\Database\Eloquent\Model {
  protected $table='review_probes'; public $timestamps=false;protected $guarded=[];protected $casts=['active'=>'boolean','meta'=>'array'];
  protected $appends=['stored_meta'];
  public function getStoredMetaAttribute(){return $this->getRawOriginal('meta');}
 }
 ReviewProbe::create(['active'=>true,'meta'=>['safe'=>true]]);return null;`
	os.WriteFile(filepath.Join(root, "bootstrap", "app.php"), []byte(bootstrap), 0600)
	svc := NewDatabaseService()
	for _, value := range []string{`{}`, `{"0":"a","1":"b"}`, `{"nested":{},"items":[{},[]]}`, `[]`} {
		t.Run("JSON shape "+value, func(t *testing.T) {
			res, err := svc.UpdateModelRecordContext(context.Background(), php, root, "ReviewProbe", 1, map[string]any{"meta": json.RawMessage(value)})
			if err != nil || !res.Success || res.Error != "" {
				t.Fatalf("update: %+v %v", res, err)
			}
			if res.Record["stored_meta"] != value {
				t.Fatalf("stored JSON = %v, want %s", res.Record["stored_meta"], value)
			}
		})
	}
	t.Run("typed values", func(t *testing.T) {
		res, err := svc.UpdateModelRecordContext(context.Background(), php, root, "ReviewProbe", 1, map[string]any{"active": false, "meta": map[string]any{"safe": false}})
		if err != nil || !res.Success || res.Error != "" {
			t.Fatalf("update: %+v %v", res, err)
		}
		meta, ok := res.Record["meta"].(map[string]any)
		if res.Record["active"] != false || !ok || meta["safe"] != false {
			t.Fatalf("cast corruption: %+v", res.Record)
		}
	})
	t.Run("NULL values", func(t *testing.T) {
		res, err := svc.UpdateModelRecordContext(context.Background(), php, root, "ReviewProbe", 1, map[string]any{"active": nil, "meta": nil})
		if err != nil || !res.Success || res.Record["active"] != nil || res.Record["meta"] != nil {
			t.Fatalf("NULL update: %+v %v", res, err)
		}
	})
	for _, attrs := range []map[string]any{{"active": "false"}, {"meta": `{"safe":false}`}} {
		t.Run("reject string coercion", func(t *testing.T) {
			res, err := svc.UpdateModelRecordContext(context.Background(), php, root, "ReviewProbe", 1, attrs)
			if err != nil || res.Success || res.Error == "" {
				t.Fatalf("invalid values accepted: %+v %v", res, err)
			}
		})
	}
}
