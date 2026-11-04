package scripts

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestCICase345SkipSourcePins(t *testing.T) {
	body := []byte("exact observed source")
	pins := map[string]string{"fixture_test.go": fmt.Sprintf("%x", sha256.Sum256(body))}
	if err := case345ValidateSourcePin(pins, "fixture_test.go", body); err != nil {
		t.Fatalf("observed source rejected: %v", err)
	}
	if err := case345ValidateSourcePin(pins, "fixture_test.go", []byte("changed skip site")); err == nil {
		t.Fatal("changed source accepted")
	}
	if err := case345ValidateSourcePin(pins, "other_test.go", body); err == nil {
		t.Fatal("unregistered source accepted")
	}
}

func TestCICase345SourceInventoryCoversGuardCalls(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   bool
	}{
		{"package x\nfunc f(){ case345Skip(t) }", true},
		{"package x\nfunc f(){ apextest.Case345Skip(t) }", true},
		{"package x\nfunc f(){ guard := apextest.Case345Skip; guard(t) }", true},
		{"package x\nvar text = `case345Skip(t)`", false},
	} {
		got, err := case345SourceNeedsPin([]byte(tc.source))
		if err != nil || got != tc.want {
			t.Errorf("source %q: got %t, %v; want %t", tc.source, got, err, tc.want)
		}
	}
	if err := case345ValidatePinnedInventory(map[string]string{"a_test.go": "hash"}, map[string]bool{}); err == nil {
		t.Fatal("deleted pinned source accepted")
	}
	if err := case345ValidatePinnedInventory(map[string]string{"a_test.go": "hash"}, map[string]bool{"a_test.go": true}); err != nil {
		t.Fatal(err)
	}
}
