package vm

import (
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONGeneratorClosedNativeAnonymous(t *testing.T) {
	dir := filepath.Join("..", "apextest", "testdata", "conformance", "json_generator_closed")
	read := func(name string) [][]string {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	bodies := map[string]string{}
	for _, row := range read("cases.tsv")[1:] {
		bodies[row[0]] = row[1]
	}
	count := 0
	for _, row := range read("org.tsv")[1:] {
		if row[1] != "anonymous" {
			continue
		}
		count++
		api, id, native := row[0], row[2], row[3]
		t.Run(api+"/"+id, func(t *testing.T) {
			body, ok := bodies[id]
			if !ok {
				t.Fatal("missing native body")
			}
			if strings.HasPrefix(native, "uncaught|") {
				source := "Boolean caught=false; try { " + body + " } catch(Exception e) { caught=true; } System.assert(false,'native error escaped');"
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := New(nil).Execute(program)
				var fault *RuntimeError
				if !errors.As(err, &fault) || fault.Type != "System.UnexpectedException" || !strings.HasPrefix(fault.Message, "Salesforce System Error") || result.Vars["caught"].Bool {
					t.Fatalf("native uncaught unexpected error; local=%v caught=%v", err, result.Vars["caught"])
				}
				return
			}
			escaped := strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n").Replace(native)
			source := "String observed='accepted'; try { " + body + " } catch(Exception e) { observed='caught|'+e.getTypeName()+'|'+e.getMessage(); } System.assertEquals('" + escaped + "',observed);"
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = New(nil).Execute(program); err != nil {
				t.Fatalf("native %s; local %v", native, err)
			}
		})
	}
	if count != 75 {
		t.Fatalf("anonymous native rows=%d, want 75", count)
	}
}
