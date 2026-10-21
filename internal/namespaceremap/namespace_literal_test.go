package namespaceremap

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestApplySourceRemapsNamespaceIdentityLiterals(t *testing.T) {
	rules := []Rule{{From: "BasePkg", To: "stagepkg"}}
	for _, tc := range []struct{ source, want string }{
		{"String prefix = 'BasePkg';", "String prefix = 'stagepkg';"},
		{"String prefix = 'basepkg';", "String prefix = 'stagepkg';"},
		{"String prefix = 'BasePkg' + '__';", "String prefix = 'stagepkg' + '__';"},
		{"String prefix = 'BasePkg' + '.';", "String prefix = 'stagepkg' + '.';"},
		{"String words = 'use BasePkg here';", "String words = 'use BasePkg here';"},
		{"String words = 'BasePkgSuffix';", "String words = 'BasePkgSuffix';"},
		{`String words = 'BasePkg\'s';`, `String words = 'BasePkg\'s';`},
		{"// String prefix = 'BasePkg';", "// String prefix = 'BasePkg';"},
		{"/* 'BasePkg' */ String prefix = 'BasePkg';", "/* 'BasePkg' */ String prefix = 'stagepkg';"},
	} {
		if got := ApplySource(rules, tc.source); got != tc.want {
			t.Errorf("ApplySource(%q) = %q, want %q", tc.source, got, tc.want)
		}
	}
}

func TestFingerprintRejectsTokenOnlySourceRemap(t *testing.T) {
	rules := []Rule{{From: "BasePkg", To: "stagepkg"}}
	legacy := sha256.Sum256([]byte("basepkg\x00stagepkg"))
	if got := Fingerprint(rules); got == hex.EncodeToString(legacy[:8]) {
		t.Fatal("source remap fingerprint accepts the prior string-token transformation")
	}
	if got := Fingerprint(nil); got != "" {
		t.Fatalf("empty-rule fingerprint = %q, want empty", got)
	}
}
