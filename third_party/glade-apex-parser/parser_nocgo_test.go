//go:build !cgo

package apexast

import "testing"

func TestNoCGODiagnostic(t *testing.T) {
	file := NewParser().ParseSource("Hello.cls", "public class Hello {}")
	if len(file.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", file.Diagnostics)
	}
	if file.Diagnostics[0].Code != "APEXPARSECGO" {
		t.Fatalf("diagnostic = %#v", file.Diagnostics[0])
	}
}
