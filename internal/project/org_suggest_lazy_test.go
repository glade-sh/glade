package project

import (
	"os"
	"os/exec"
	"regexp"
	"sync"
	"testing"
)

func TestSetupRegexpLazyConcurrent(t *testing.T) {
	const pattern = `(?i)\bSite\.(login|forgotPassword)\b`
	lazy := newSetupRegexp(pattern)
	if lazy.compiled != nil {
		t.Fatal("compiled before use")
	}
	want := regexp.MustCompile(pattern)
	var first *regexp.Regexp
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Go(func() {
			got := lazy.get()
			mu.Lock()
			if first == nil {
				first = got
			} else if first != got {
				t.Error("compiled more than once")
			}
			mu.Unlock()
			for _, input := range []string{"Site.login", "site.forgotPassword", "Site.notLogin", "prefixSite.login", ""} {
				if got.MatchString(input) != want.MatchString(input) {
					t.Errorf("different match for %q", input)
				}
			}
		})
	}
	wg.Wait()
}

func TestSetupPatternsStartUninitialized(t *testing.T) {
	if os.Getenv("GLADE_SETUP_PATTERNS_CHILD") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestSetupPatternsStartUninitialized$")
		cmd.Env = append(os.Environ(), "GLADE_SETUP_PATTERNS_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fresh process: %v\n%s", err, output)
		}
		return
	}
	if setupStringRE.compiled != nil || currencyTypeRE.compiled != nil {
		t.Fatal("setup patterns initialized at startup")
	}
	if setupStringRE.get() == nil || currencyTypeRE.get() == nil {
		t.Fatal("setup patterns not initialized on use")
	}
}
