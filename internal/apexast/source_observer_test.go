package apexast

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsersNotifySourceObserver(t *testing.T) {
	var seen []string
	previous := SetSourceObserver(func(source string) { seen = append(seen, source) })
	defer SetSourceObserver(previous)

	parser := NewParser()
	defer parser.Close()
	parser.ParseSource("A.cls", "public class A {}")
	parser.ParseSourceAST("B.cls", "public class B {}")
	ParseSource("C.cls", "public class C {}")
	ParseSourceAST("D.cls", "public class D {}")
	path := filepath.Join(t.TempDir(), "E.cls")
	if err := os.WriteFile(path, []byte("public class E {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(path); err != nil {
		t.Fatal(err)
	}
	want := []string{"public class A {}", "public class B {}", "public class C {}", "public class D {}", "public class E {}"}
	if len(seen) != len(want) {
		t.Fatalf("observer saw %d sources %q, want %q", len(seen), seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("source %d = %q, want %q", i, seen[i], want[i])
		}
	}
	SetSourceObserver(nil)
	ParseSource("F.cls", "public class F {}")
	if len(seen) != len(want) {
		t.Fatal("removed observer still notified")
	}
}
