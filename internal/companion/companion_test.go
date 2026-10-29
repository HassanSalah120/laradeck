package companion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingIdentitySurvivesMovementButRequiresReviewAfterSourceChanges(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "service.php")
	code := "<?php\nfunction save() {\n  $a->save();\n  $b->save();\n}\n"
	os.WriteFile(path, []byte(code), 0600)
	before := Fingerprint(root, "service.php", "transaction", "candidate", 3)
	os.WriteFile(path, []byte("// inserted\n\n"+code), 0600)
	moved := Fingerprint(root, "service.php", "transaction", "candidate", 5)
	if before != moved {
		t.Fatal("line movement changed identity")
	}
	os.WriteFile(path, []byte(strings.Replace(code, "$b->save();", "DB::transaction(fn()=> $b->save());", 1)), 0600)
	if before == Fingerprint(root, "service.php", "transaction", "candidate", 3) {
		t.Fatal("source change retained disposition")
	}
}
func TestComparisonAccountsForRepeatedQueryCountsAndMissingMemory(t *testing.T) {
	a := Performance{Method: "GET", URI: "/projects", Duration: 100, QueryCount: 1, Queries: []string{"select * from projects where id = 1"}}
	b := a
	b.Duration = 140
	b.QueryCount = 3
	b.Queries = []string{"select * from projects where id = 2", "select * from projects where id = 3", "select * from tasks where owner_id = 4"}
	result, err := Compare(a, b)
	if err != nil || result.DurationDelta != 40 || result.QueryDelta != 2 || len(result.Added) != 2 || result.MemoryDelta != nil {
		t.Fatalf("incorrect comparison: %+v %v", result, err)
	}
	b.URI = "/other"
	if _, err = Compare(a, b); err == nil {
		t.Fatal("different route compared")
	}
}
func TestArtifactsRemainConfinedAndReplaceAtomically(t *testing.T) {
	s := NewStore(t.TempDir())
	for _, id := range []string{"../outside", "a/b", "C:\\outside", ""} {
		if _, err := s.Path(id, "profile"); err == nil {
			t.Fatalf("accepted %q", id)
		}
	}
	for _, value := range []string{"first", "second"} {
		if err := s.Save("project", "profile", map[string]string{"name": value}); err != nil {
			t.Fatal(err)
		}
	}
	var result map[string]string
	if err := s.Load("project", "profile", &result); err != nil || result["name"] != "second" {
		t.Fatalf("artifact replacement failed: %v %v", result, err)
	}
}
func TestHistoryDoesNotCallMissingCoverageResolved(t *testing.T) {
	changes := Diff([]Finding{{Fingerprint: "stable", Rule: "r"}}, nil)
	if len(changes) != 1 || changes[0].Status != "not_observed" {
		t.Fatalf("unsupported resolution claim: %+v", changes)
	}
}

func TestQueryNormalizationPreservesPostgresIdentifiers(t *testing.T) {
	if NormalizeQuery(`select * from "users" where "id" = 1`) == NormalizeQuery(`select * from "patients" where "id" = 2`) {
		t.Fatal("different tables normalized to the same query")
	}
}
