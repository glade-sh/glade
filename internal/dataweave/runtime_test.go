package dataweave

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProtocolPreservesSourceVersionAndDecimal(t *testing.T) {
	req := Request{Name: "owned", APIVersion: "67.0", Source: "%dw 2.0\n---\npayload", Inputs: map[string]Input{"payload": {MIMEType: "application/json", Data: []byte(`{"n":1234567890123456789012345678.125,"missing":null}`)}}}
	first, err := encodeRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(first, req.Inputs["payload"].Data) {
		t.Fatal("input was changed")
	}
	req.APIVersion = "66.0"
	second, err := encodeRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("API version did not affect request identity")
	}
	invalid := req
	invalid.Source = string([]byte{0xff})
	if _, err := encodeRequest(invalid); err == nil {
		t.Fatal("invalid UTF8 source accepted")
	}

	for _, data := range [][]byte{nil, {2}, {0, 0, 0, 0, 255}} {
		if _, err := decodeResult(data); err == nil {
			t.Fatalf("accepted invalid response %x", data)
		}
	}
}

// The integration test needs a provisioned, verified engine graph. It never
// downloads dependencies. CI/toolchain provisioning owns platform availability.
func TestOfficialEngineSourceAndDenials(t *testing.T) {
	cp := os.Getenv("GLADE_DATAWEAVE_TEST_CLASSPATH")
	javaHome := os.Getenv("GLADE_DATAWEAVE_TEST_JAVA_HOME")
	if cp == "" || javaHome == "" {
		t.Skip("explicit DataWeave engine and Java17 test toolchain required")
	}
	rt := Runtime{JavaPath: filepath.Join(javaHome, "bin", "java"), ClassPath: filepath.SplitList(cp), AdapterDirectory: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := CompileAdapter(ctx, filepath.Join(javaHome, "bin", "javac"), rt.ClassPath, rt.AdapterDirectory); err != nil {
		t.Fatal(err)
	}
	input := map[string]Input{"payload": {MIMEType: "application/json", Data: []byte(`{"left":3,"right":4,"text":"first","n":1.25,"missing":null}`)}}
	execute := func(name, source string) (Result, error) {
		return rt.Execute(ctx, Request{Name: name, Source: source, APIVersion: "67.0", Inputs: input})
	}
	source := "%dw 2.0\noutput application/json\n---\npayload.left + payload.right"
	for _, name := range []string{"helloWorld", "ownedRenamed"} {
		r, err := execute(name, source)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(r.Data)) != "7" || r.MIMEType != "application/json" || r.RequestSHA256 == "" {
			t.Fatalf("unexpected result: %+v", r)
		}
	}
	changed, err := execute("helloWorld", strings.Replace(source, "payload.right", "payload.right + 5", 1))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(changed.Data)) != "12" {
		t.Fatalf("changed source: %s", changed.Data)
	}
	imported, err := execute("ownedImport", "%dw 2.0\nimport upper from dw::Core\noutput application/json\n---\n{text:upper(payload.text), n:payload.n, missing:payload.missing}")
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]any
	if err := json.Unmarshal(imported.Data, &actual); err != nil {
		t.Fatal(err)
	}
	if actual["text"] != "FIRST" || actual["n"] != 1.25 || actual["missing"] != nil {
		t.Fatalf("import: %s", imported.Data)
	}
	owned := filepath.Join(t.TempDir(), "owned.txt")
	if err := os.WriteFile(owned, []byte("owned-denial-marker"), 0600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"file": "readUrl(\"file://" + filepath.ToSlash(owned) + "\",\"text/plain\")",
		"env":  "dw::System::envVar(\"PATH\")",
		"java": "java!java::lang::System::getProperty(\"java.version\")",
	}
	for name, expr := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := execute(name, "%dw 2.0\noutput application/json\n---\n"+expr)
			if err == nil {
				t.Fatal("forbidden operation succeeded")
			}
			var engine *EngineError
			if !errors.As(err, &engine) {
				t.Fatalf("not an engine diagnostic: %v", err)
			}
		})
	}
	t.Run("xmlValue", func(t *testing.T) {
		result, err := rt.Execute(ctx, Request{Name: "xmlValue", APIVersion: "67.0", Source: "%dw 2.0\noutput application/json\n---\npayload.root.value", Inputs: map[string]Input{"payload": {MIMEType: "application/xml", Data: []byte("<root><value>owned</value></root>")}}})
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(result.Data)) != `"owned"` {
			t.Fatalf("XML value: %s", result.Data)
		}
	})
	t.Run("decimalPrecision", func(t *testing.T) {
		const decimal = "1234567890123456789012345678.125"
		result, err := rt.Execute(ctx, Request{Name: "precise", APIVersion: "67.0", Source: "%dw 2.0\noutput application/json\n---\npayload.n", Inputs: map[string]Input{"payload": {MIMEType: "application/json", Data: []byte(`{"n":` + decimal + `}`)}}})
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(result.Data)) != decimal {
			t.Fatalf("decimal changed: %s", result.Data)
		}
	})

	t.Run("xmlExternalEntity", func(t *testing.T) {
		xml := `<!DOCTYPE root [<!ENTITY ext SYSTEM "file://` + filepath.ToSlash(owned) + `">]><root>&ext;</root>`
		_, err := rt.Execute(ctx, Request{Name: "xmlEntity", APIVersion: "67.0", Source: "%dw 2.0\noutput application/json\n---\npayload", Inputs: map[string]Input{"payload": {MIMEType: "application/xml", Data: []byte(xml)}}})
		if err == nil {
			t.Fatal("external entity accepted")
		}
		var diagnostic *EngineError
		if !errors.As(err, &diagnostic) {
			t.Fatalf("external entity host failure: %v", err)
		}
	})
	t.Run("customClasspathModule", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(rt.AdapterDirectory, "OwnedCustom.dwl"), []byte("%dw 2.0\nfun secret() = \"owned-marker\""), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := execute("custom", "%dw 2.0\nimport secret from OwnedCustom\noutput application/json\n---\nsecret()")
		if err == nil {
			t.Fatal("custom classpath module accepted")
		}
	})

	t.Run("builtinNamespaceInjection", func(t *testing.T) {
		dir := filepath.Join(rt.AdapterDirectory, "dw")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "OwnedCustom.dwl"), []byte("%dw 2.0\nfun secret() = \"owned-custom-marker\""), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := execute("builtinInjection", "%dw 2.0\nimport secret from dw::OwnedCustom\noutput application/json\n---\nsecret()")
		if err == nil {
			t.Fatal("adapter-directory builtin namespace injection accepted")
		}
	})
	t.Run("sourceDeclaredInputs", func(t *testing.T) {
		for _, item := range []struct{ mime, data, expression, want string }{{"application/json", `{"left":3,"right":4}`, "payload.left + payload.right", "7"}, {"application/csv", "name,amount\nowned,3\n", "payload[0].name", `"owned"`}, {"text/plain", "owned plain", `payload ++ " value"`, `"owned plain value"`}} {
			result, err := rt.Execute(ctx, Request{Name: "declared", APIVersion: "67.0", Source: "%dw 2.0\ninput payload " + item.mime + "\noutput application/json\n---\n" + item.expression, Inputs: map[string]Input{"payload": {Data: []byte(item.data)}}})
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(result.Data)) != item.want {
				t.Fatalf("declared %s: %s", item.mime, result.Data)
			}
		}
	})
	t.Run("outputLimitIsHostFailure", func(t *testing.T) {
		_, err := rt.Execute(ctx, Request{Name: "outputBound", APIVersion: "67.0", Source: "%dw 2.0\ninput payload text/plain\noutput text/plain\n---\npayload ++ payload", Inputs: map[string]Input{"payload": {Data: bytes.Repeat([]byte("x"), (8<<20)+1)}}})
		var host *HostError
		if !errors.As(err, &host) || host.Kind != "output-limit" {
			t.Fatalf("output bound was not host error: %T %v", err, err)
		}
	})

	t.Run("changedClasspathFailsBeforeJava", func(t *testing.T) {
		changed := rt
		changed.ClassPath = append([]string(nil), rt.ClassPath...)
		data, err := os.ReadFile(changed.ClassPath[0])
		if err != nil {
			t.Fatal(err)
		}
		data[0] ^= 1
		dest := filepath.Join(t.TempDir(), filepath.Base(changed.ClassPath[0]))
		if err := os.WriteFile(dest, data, 0600); err != nil {
			t.Fatal(err)
		}
		changed.ClassPath[0] = dest
		_, err = changed.Execute(ctx, Request{Name: "tampered", APIVersion: "67.0", Source: source, Inputs: input})
		var host *HostError
		if !errors.As(err, &host) || host.Kind != "toolchain" {
			t.Fatalf("changed classpath accepted: %v", err)
		}
	})

	_, err = execute("broken", "%dw 2.0\n---\n(")
	var engine *EngineError
	if !errors.As(err, &engine) || engine.Phase != "compile" {
		t.Fatalf("compiler diagnostic: %v", err)
	}
	rt.Timeout = time.Millisecond
	_, err = rt.Execute(context.Background(), Request{Name: "bounded", Source: source, APIVersion: "67.0", Inputs: input})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline: %v", err)
	}
}
