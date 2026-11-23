package vm_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/vm"
)

func TestValuePersistedCacheMatchesParent(t *testing.T) {
	if os.Getenv("GLADE_VALUE_CACHE_ORACLE") != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestValuePersistedCacheMatchesParent$", "-test.v")
		command.Env = append(os.Environ(), "GLADE_VALUE_CACHE_ORACLE=1")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("cache byte oracle: %s: %v", out, err)
		}
		t.Logf("%s", out)
		return
	}
	values := []vm.Value{
		{},
		{Kind: vm.ValueObject, Type: "Record", Fields: map[string]vm.Value{"Name": {Kind: vm.ValueString, Text: "text"}}},
		{Kind: vm.ValueList, List: []vm.Value{{Kind: vm.ValueInt, Int: 42}, {Kind: vm.ValueMap, Map: map[string]vm.Value{"key": {Kind: vm.ValueBool, Bool: true}}}}},
	}
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(values); err != nil {
		t.Fatal(err)
	}
	program, err := vm.CompileAnonymous(`List<Object> values = new List<Object>{'text', 42, null}; System.debug(JSON.serialize(values));`)
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
	valueHash, cacheHash := fmt.Sprintf("%x", sha256.Sum256(buffer.Bytes())), fmt.Sprintf("%x", sha256.Sum256(data))
	if valueHash != "44685425f08834dc2b3ef8c9d9d72ba9f4001df0747dd9af05711a0efe0b4f95" || cacheHash != "79b83230e729916d6eb0acfde80f175fd9661ae48986360cf0a874ec689cc6fc" {
		t.Fatalf("parent byte digests: gob=%s cache=%s", valueHash, cacheHash)
	}
}
