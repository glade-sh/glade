package sema

import (
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reference functions below are the per-offset scanners that
// semaIgnoredText replaced, kept verbatim as the oracle.

func referenceSemaOffsetInIgnoredText(body string, pos int) bool {
	if referenceSemaOffsetInSOQLLiteral(body, pos) {
		return true
	}
	inBlock := false
	for i := 0; i < len(body) && i < pos; i++ {
		if inBlock {
			if i+1 < len(body) && body[i] == '*' && body[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}
		if body[i] == '\'' {
			end := skipSemaString(body, i)
			if pos <= end {
				return true
			}
			i = end
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '*' {
			inBlock = true
			i++
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '/' {
			lineEnd := strings.IndexAny(body[i+2:], "\r\n")
			if lineEnd < 0 || i+2+lineEnd >= pos {
				return true
			}
			i += 2 + lineEnd
		}
	}
	if inBlock {
		return true
	}
	return false
}

func referenceSemaOffsetInParenGroup(body string, pos int) bool {
	depth := 0
	inBlock := false
	for i := 0; i < len(body) && i < pos; i++ {
		if inBlock {
			if i+1 < len(body) && body[i] == '*' && body[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}
		if body[i] == '\'' {
			i = skipSemaString(body, i)
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '*' {
			inBlock = true
			i++
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '/' {
			for i < len(body) && body[i] != '\n' {
				i++
			}
			continue
		}
		switch body[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth > 0
}

func referenceSemaOffsetInSOQLLiteral(body string, pos int) bool {
	for i := 0; i < len(body) && i < pos; i++ {
		switch body[i] {
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
		case '\'':
			i = skipSemaString(body, i)
		case '[':
			queryStart := i + 1
			for queryStart < len(body) && isWhitespace(body[queryStart]) {
				queryStart++
			}
			if !strings.HasPrefix(strings.ToLower(body[queryStart:]), "select") && !strings.HasPrefix(strings.ToLower(body[queryStart:]), "find") {
				continue
			}
			depth := 1
			for j := i + 1; j < len(body); j++ {
				switch body[j] {
				case '/':
					if end, ok := skipSemaComment(body, j); ok {
						j = end
					}
				case '\'':
					j = skipSemaString(body, j)
				case '[':
					depth++
				case ']':
					depth--
					if depth == 0 {
						if pos > i && pos < j {
							return true
						}
						i = j
						j = len(body)
					}
				}
			}
		}
	}
	return false
}

func assertSemaIgnoredTextMatchesReference(t testing.TB, body string) {
	t.Helper()
	ignored := newSemaIgnoredText(body)
	for pos := -2; pos <= len(body)+2; pos++ {
		if got, want := ignored.contains(pos), referenceSemaOffsetInIgnoredText(body, pos); got != want {
			t.Fatalf("contains(%d) = %v, want %v for body %q", pos, got, want, body)
		}
		if got, want := ignored.inSOQLLiteral(pos), referenceSemaOffsetInSOQLLiteral(body, pos); got != want {
			t.Fatalf("inSOQLLiteral(%d) = %v, want %v for body %q", pos, got, want, body)
		}
		if got, want := ignored.inParenGroup(pos), referenceSemaOffsetInParenGroup(body, pos); got != want {
			t.Fatalf("inParenGroup(%d) = %v, want %v for body %q", pos, got, want, body)
		}
	}
}

var semaIgnoredTextEdgeBodies = []string{
	"",
	"'",
	"/",
	"/*",
	"/*/",
	"/**/",
	"/*/ x */ y",
	"//",
	"// no newline",
	"a // c\r\nb",
	"a // c\rb // d\n",
	"x = 'it''s' + 'a\\'b' + 'open",
	"x = '\\",
	"[select Id from Account]",
	"[ SELECT Id FROM Account WHERE Name = 'a]b' ]",
	"[\n\tFIND 'x' IN ALL FIELDS RETURNING Account]",
	"[select [select Id from Contact] from Account]",
	"[select [select Id from Contact] from Account",
	"[select Id from A] [select Id from B",
	"x[0] + y[1] + [select Id /* ] */ from A // ]\n ]",
	"[selec Id] [fin x] [findx] [selectx]",
	"[FİND x]",
	"[İselect]",
	"[\xffselect]",
	"[sel\xffect x]",
	"[SKelect]",
	"foo(a, (b), 'c)', /* ( */ d // (\n )",
	"if (x) { y(); } ) ) (",
	"a(/* unterminated",
	"a('unterminated (",
	"a(// line ( \r b) c",
	"List<Account> xs = [select Id from Account where Name in :names];\nfor (Account a : xs) { System.debug(a); }",
}

func TestSemaIgnoredTextMatchesReferenceOnEdgeCases(t *testing.T) {
	for _, body := range semaIgnoredTextEdgeBodies {
		assertSemaIgnoredTextMatchesReference(t, body)
	}
}

func TestSemaIgnoredTextMatchesReferenceOnRandomBodies(t *testing.T) {
	pieces := []string{
		"/", "*", "'", "\\", "[", "]", "(", ")", "\n", "\r", " ", "\t",
		"select", "SELECT", "find", "Find", "x", "=", ";", "İ", "\xff", "//", "/*", "*/", "''",
	}
	rng := rand.New(rand.NewSource(1))
	for n := 0; n < 4000; n++ {
		var b strings.Builder
		for k := rng.Intn(40); k >= 0; k-- {
			b.WriteString(pieces[rng.Intn(len(pieces))])
		}
		assertSemaIgnoredTextMatchesReference(t, b.String())
	}
}

func TestSemaIgnoredTextMatchesReferenceOnRepositoryApex(t *testing.T) {
	root := filepath.Join("..", "..")
	checked := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == "node_modules" || entry.Name() == ".git") {
			return filepath.SkipDir
		}
		if entry.IsDir() || (!strings.HasSuffix(path, ".cls") && !strings.HasSuffix(path, ".trigger")) {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(source) > 64<<10 {
			return nil
		}
		assertSemaIgnoredTextMatchesReference(t, string(source))
		checked++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no Apex sources found")
	}
}

func FuzzSemaIgnoredTextMatchesReference(f *testing.F) {
	for _, body := range semaIgnoredTextEdgeBodies {
		f.Add(body)
	}
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 4<<10 {
			return
		}
		assertSemaIgnoredTextMatchesReference(t, body)
	})
}

// The per-offset scanners lowered the rest of the body at every '[' before the
// offset, so a body with many index expressions cost O(brackets x length)
// allocations per query. The shared index scans once.
func TestSemaIgnoredTextQueriesDoNotRescanBody(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 400; i++ {
		b.WriteString("x[0] = new Foo(y[1]); // c\n")
	}
	body := b.String()
	ignored := newSemaIgnoredText(body)
	ignored.contains(0)
	ignored.inParenGroup(0)
	allocs := testing.AllocsPerRun(5, func() {
		for pos := 0; pos < len(body); pos += 7 {
			ignored.contains(pos)
			ignored.inSOQLLiteral(pos)
			ignored.inParenGroup(pos)
		}
	})
	if allocs != 0 {
		t.Fatalf("queries on a built index allocated %.0f times; want 0", allocs)
	}
	build := testing.AllocsPerRun(5, func() {
		fresh := newSemaIgnoredText(body)
		fresh.contains(len(body) / 2)
		fresh.inParenGroup(len(body) / 2)
	})
	if build > 64 {
		t.Fatalf("building the index allocated %.0f times; want at most 64", build)
	}
	tokens := testing.AllocsPerRun(3, func() {
		constructorTypes(body)
	})
	// One token per constructor plus regexp and index overhead; the
	// per-offset scan allocated about 2 x 800 lowered copies per match.
	if tokens > 4*400+256 {
		t.Fatalf("constructorTypes allocated %.0f times for 400 constructors; want at most %d", tokens, 4*400+256)
	}
}
