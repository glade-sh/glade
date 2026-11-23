package vm_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/vm"
)

func TestJSONParserDiskCachePayloadIdentity(t *testing.T) {
	// Gob assigns type IDs process-wide. Isolate this byte oracle from other tests.
	if os.Getenv("GLADE_PARSER_CACHE_ORACLE") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command(executable, "-test.run=^TestJSONParserDiskCachePayloadIdentity$")
		command.Env = append(os.Environ(), "GLADE_PARSER_CACHE_ORACLE=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("cache byte oracle: %s: %v", output, err)
		}
		return
	}
	program, err := vm.CompileAnonymous(`JSONParser p=JSON.createParser('{"a":[1,true,null]}'); while(p.nextToken()!=null) { p.getText(); }`)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	entry := startupcache.Entry{ProjectRoot: root, Runtime: startupcache.CompiledRuntime{Methods: map[string]vm.Method{"exercise": {Name: "exercise", Program: program}}}}
	if _, err := startupcache.WriteWithStats(&entry, startupcache.SubdirTest); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(root, startupcache.SubdirTest, "startup.payload.*.gob"))
	if err != nil || len(files) != 1 {
		t.Fatalf("payload files %v: %v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := json.Marshal(entry.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(jsonData)); got != "1d2ea948b35bf7c842ebfc80ff7b884941f2f961ba77d832a4815efde0b0d237" {
		t.Fatalf("debug runtime digest %s", got)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	if got != "3639cab4a7f2b8a99c92cd5f2cc9f3ab3f7b59f027b789b7c9c627bd2982f2f7" {
		t.Fatalf("disk payload digest %s", got)
	}
}
