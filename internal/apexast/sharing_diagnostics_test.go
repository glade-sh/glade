package apexast

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// referenceSharingDeclarationDiagnostics runs the pattern unconditionally.
func referenceSharingDeclarationDiagnostics(path, source string) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	lines := NewLineMap(source)
	for _, match := range sharingDeclarationPattern.FindAllStringSubmatchIndex(source, -1) {
		message := ""
		switch {
		case match[2] >= 0:
			message = "Unexpected token 'class'."
		case match[4] >= 0:
			message = "withSharing classes cannot be withoutSharing"
		case match[6] >= 0:
			message = "inheritedSharing classes cannot be withSharing"
		default:
			continue
		}
		rng := diagnostic.Range{Start: lines.Position(match[0]), End: lines.Position(match[1])}
		out = append(out, diagnostic.Diagnostic{
			Severity: diagnostic.Error, Code: "APEXPARSE001", File: path,
			Message: message, NativeMessage: message, Range: &rng,
		})
	}
	return out
}

var sharingDeclarationSamples = []string{
	"public with sharing class A {}\n",
	"public without sharing class A {}\n",
	"public inherited sharing class A {}\n",
	"public class A {}\n",
	"public class A with sharing {}\n",
	"public class A without sharing {}\n",
	"public class A inherited sharing {}\n",
	"public class\tA\n\twithout\n\tsharing {}\n",
	"public with sharing without sharing class A {}\n",
	"public inherited sharing with sharing class A {}\n",
	"public with sharing with sharing class A {}\n",
	"// class A with sharing\npublic with sharing class A {}\n",
	"/* class A with sharing */ public with sharing class A {}\n",
	"public with sharing class A { String s = 'class B with sharing'; }\n",
	"public with sharing class A { String s = 'unterminated class B with sharing\n",
	"public with sharing class A { /* unterminated class B with sharing\n",
	"public class Asharing with sharing {}\n",
	"public class A withsharing {}\n",
	"public class A with  sharingX {}\n",
	"public class A with sharing9 {}\n",
	"public with sharing class A { static void m() { String sharing = 'x'; } }\n",
	"public class A { String with; } sharing\n",
	"class_ A with sharing\n",
	"xclass A with sharing\n",
	"class 9A with sharing\n",
	"\tclass A with\fsharing\n",
	"class A with\vsharing\n",
	"Class A With Sharing\n",
	"",
	"sharing",
	"with sharing",
	"class A with sharing",
}

func TestSharingDeclarationDiagnosticsMatchPattern(t *testing.T) {
	check := func(t *testing.T, name, source string) {
		t.Helper()
		want := referenceSharingDeclarationDiagnostics(name, source)
		got := SharingDeclarationDiagnostics(name, source)
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("%s: diagnostics differ\nwant %+v\ngot  %+v", name, want, got)
		}
	}
	for i, source := range sharingDeclarationSamples {
		check(t, "sample"+string(rune('A'+i)), source)
	}
	roots := []string{filepath.Join("..", "..")}
	if extra := os.Getenv("GLADE_SEMA_EQUIV_DIRS"); extra != "" {
		roots = append(roots, strings.Split(extra, string(os.PathListSeparator))...)
	}
	files, candidates := 0, 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if base := entry.Name(); base == "node_modules" || base == ".git" || base == "web" {
					return filepath.SkipDir
				}
				return nil
			}
			if ext := filepath.Ext(path); ext != ".cls" && ext != ".trigger" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			files++
			if sharingDeclarationCandidate(string(data)) {
				candidates++
			}
			check(t, path, string(data))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if files == 0 {
		t.Fatal("no Apex sources compared")
	}
	t.Logf("compared %d files; %d ran the pattern", files, candidates)
}
