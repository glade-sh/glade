package gladecli

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestRunExecWithDBUnchangedDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	path := filepath.Join(root, ".glade", "envs", "dev.sqlite")
	run := func(source string) {
		t.Helper()
		var out, errout bytes.Buffer
		if code := Run(context.Background(), []string{"exec", "--project", root, "--db", path, source}, &out, &errout); code != 0 {
			t.Fatalf("exec code=%d stdout=%q stderr=%q", code, out.String(), errout.String())
		}
	}
	run("Integer x = 1;")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"org_meta", "object_definitions", "records", "id_sequences"} {
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE"} {
			if _, err := db.Exec("CREATE TRIGGER no_write_" + table + "_" + operation + " BEFORE " + operation + " ON " + table + " BEGIN SELECT RAISE(ABORT, 'unexpected write'); END"); err != nil {
				t.Fatal(err)
			}
		}
	}
	run("Integer x = 1;")
}

func BenchmarkExecDBReadOnly(b *testing.B) {
	root := b.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`), 0600); err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(root, "org.sqlite")
	args := []string{"exec", "--project", root, "--db", path, "Integer x = 1;"}
	run := func() {
		var out, errout bytes.Buffer
		if code := Run(context.Background(), args, &out, &errout); code != 0 {
			b.Fatalf("exec code=%d stderr=%q", code, errout.String())
		}
	}
	run()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run()
	}
}
