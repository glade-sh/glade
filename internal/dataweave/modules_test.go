package dataweave

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficialEngineExplicitProjectModules(t *testing.T) {
	cp, javaHome := os.Getenv("GLADE_DATAWEAVE_TEST_CLASSPATH"), os.Getenv("GLADE_DATAWEAVE_TEST_JAVA_HOME")
	if cp == "" || javaHome == "" {
		t.Skip("explicit DataWeave engine and Java17 test toolchain required")
	}
	rt := Runtime{JavaPath: filepath.Join(javaHome, "bin", "java"), ClassPath: filepath.SplitList(cp), AdapterDirectory: t.TempDir()}
	ctx := context.Background()
	if err := CompileAdapter(ctx, filepath.Join(javaHome, "bin", "javac"), rt.ClassPath, rt.AdapterDirectory); err != nil {
		t.Fatal(err)
	}
	// Exact source/input/API composition from the admitted owned-module proof.
	request := Request{Name: "gladeC5Custom", APIVersion: "67.0", Source: "%dw 2.0\nimport ownedSum from gladeC5OwnedModule\ninput payload application/json\noutput application/json indent=false\n---\nownedSum(payload.left, payload.right)\n", Inputs: map[string]Input{"payload": {Data: []byte(`{"left":3,"right":4}`)}}, Modules: map[string]Module{"gladeC5OwnedModule": {Name: "gladeC5OwnedModule", APIVersion: "67.0", Source: "%dw 2.0\nfun ownedSum(left, right) = left + right\n"}}}
	result, err := rt.Execute(ctx, request)
	if err != nil || string(result.Data) != "7" {
		t.Fatalf("explicit module result=%+v err=%v", result, err)
	}
	originalIdentity := result.RequestSHA256
	module := request.Modules["gladeC5OwnedModule"]
	module.Source = strings.Replace(module.Source, "left + right", "left + right + 5", 1)
	request.Modules["gladeC5OwnedModule"] = module
	result, err = rt.Execute(ctx, request)
	if err != nil || string(result.Data) != "12" || result.RequestSHA256 == originalIdentity {
		t.Fatalf("changed module result=%+v err=%v", result, err)
	}
	// Module code runs under the same denied host privileges as main source.
	module.Source = "%dw 2.0\nfun ownedSum(left, right) = dw::System::envVar(\"PATH\")\n"
	request.Modules["gladeC5OwnedModule"] = module
	if _, err := rt.Execute(ctx, request); err == nil {
		t.Fatal("project module escaped environment denial")
	} else if engine, ok := err.(*EngineError); !ok || engine.Phase != "execute" || !strings.Contains(engine.Message, "Environment") {
		t.Fatalf("module denial did not reach execute privilege check: %#v", err)
	}
	delete(request.Modules, "gladeC5OwnedModule")
	if _, err := rt.Execute(ctx, request); err == nil {
		t.Fatal("module from previous request leaked")
	}
}

func TestProjectModuleIdentityAndValidation(t *testing.T) {
	request := Request{Name: "owned", APIVersion: "67.0", Source: "%dw 2.0\n---\n7", Modules: map[string]Module{"ownedModule": {Name: "ownedModule", Namespace: "one", APIVersion: "67.0", Source: "%dw 2.0\nfun owned() = 7\n"}}}
	initial, err := encodeRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"name", "namespace", "version", "source"} {
		t.Run(field, func(t *testing.T) {
			changed := request.Modules["ownedModule"]
			switch field {
			case "name":
				changed.Name = "renamed"
			case "namespace":
				changed.Namespace = "two"
			case "version":
				changed.APIVersion = "66.0"
			case "source":
				changed.Source += "\n"
			}
			copy := request
			copy.Modules = map[string]Module{"ownedModule": changed}
			encoded, err := encodeRequest(copy)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(initial, encoded) {
				t.Fatalf("module %s omitted from identity", field)
			}
		})
	}
	for _, module := range []Module{{Name: "", Source: "7"}, {Name: "owned", Source: string([]byte{0xff})}} {
		request.Modules = map[string]Module{"ownedModule": module}
		if _, err := encodeRequest(request); err == nil {
			t.Fatal("invalid module accepted")
		}
	}
}
