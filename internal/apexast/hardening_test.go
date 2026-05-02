package apexast

import "testing"

func TestNoPanicOnMalformedSources(t *testing.T) {
	sources := []string{
		"",
		"public class Broken {",
		"trigger Broken on Account (before insert",
		"public class Broken { public void run( {",
		"@@@",
	}
	for _, source := range sources {
		assertNoPanic(t, func() {
			_ = NewParser().ParseSource("Broken.cls", source)
		})
	}
}

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}
