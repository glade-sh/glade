package project

import (
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
