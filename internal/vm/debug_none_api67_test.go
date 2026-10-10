package vm

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"
)

// This API 67 anonymous source is the blank-namespace Salesforce fixture for
// the base System runtime context (source SHA-256
// eab45ce9f10ecd9883db2084bd63d7eb4453f48e4fe3aa4e1b30ee6ca12788cf).
// Its observed debug vector omits only the LoggingLevel.NONE call at line 11.
func TestExecSystemDebugNoneSuppressesEmissionAPI67(t *testing.T) {
	t.Run("observed ordered vector", func(t *testing.T) {
		source := `System.assert(true);
System.assert(true, 'assert message');
System.assertEquals(1, 1);
System.assertEquals(1, 1, 'equals message');
System.assertNotEquals(1, 2);
System.assertNotEquals(1, 2, 'not equals message');
System.debug(LoggingLevel.ERROR, 'error');
System.debug(LoggingLevel.WARN, 'warn');
System.debug(LoggingLevel.INFO, 'info');
System.debug(LoggingLevel.DEBUG, 'debug');
System.debug(LoggingLevel.NONE, 'none');
System.debug('single object');
System.assertEquals(LoggingLevel.INFO, LoggingLevel.INFO);
System.assertEquals(Date.today(), System.today());
System.assertNotEquals(null, System.now());
System.assert(System.currentTimeMillis() > 0);
System.assertEquals(false, System.isBatch());
System.assertEquals(false, System.isFuture());
System.assertEquals(false, System.isQueueable());
System.assertEquals(false, System.isScheduled());`
		if got := len(source); got != 847 {
			t.Fatalf("fixture source bytes = %d, want 847", got)
		}
		sourceHash := sha256.Sum256([]byte(source))
		if got := hex.EncodeToString(sourceHash[:]); got != "eab45ce9f10ecd9883db2084bd63d7eb4453f48e4fe3aa4e1b30ee6ca12788cf" {
			t.Fatalf("fixture source SHA-256 = %s", got)
		}
		program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
		if err != nil {
			t.Fatal(err)
		}
		if program.APIVersion != "67.0" {
			t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
		}

		machine := New(nil)
		machine.SetTraceEnabled(true)
		var stdout bytes.Buffer
		machine.Stdout = &stdout
		var sink []DebugEvent
		machine.SetDebugOutputSink(func(event DebugEvent) { sink = append(sink, event) })
		result, err := machine.Execute(program)
		if err != nil {
			t.Fatal(err)
		}

		wantMessages := []string{"error", "warn", "info", "debug", "single object"}
		if !reflect.DeepEqual(result.Debug, wantMessages) {
			t.Fatalf("debug messages = %#v, want %#v", result.Debug, wantMessages)
		}
		wantLevels := []string{"ERROR", "WARN", "INFO", "DEBUG", "DEBUG"}
		assertDebugEventVector(t, "trace events", result.DebugEvents, wantLevels, wantMessages)
		assertDebugEventVector(t, "sink callbacks", sink, wantLevels, wantMessages)
		if got, want := stdout.String(), "error\nwarn\ninfo\ndebug\nsingle object\n"; got != want {
			t.Fatalf("debug stdout = %q, want %q", got, want)
		}
	})

	t.Run("none still evaluates its message argument", func(t *testing.T) {
		program, err := CompileAnonymousWithOptions(`Integer evaluated = 0;
System.debug(LoggingLevel.NONE, ++evaluated);
System.assertEquals(1, evaluated);`, CompileOptions{APIVersion: "67.0"})
		if err != nil {
			t.Fatal(err)
		}
		machine := New(nil)
		machine.SetTraceEnabled(true)
		var stdout bytes.Buffer
		machine.Stdout = &stdout
		var sink []DebugEvent
		machine.SetDebugOutputSink(func(event DebugEvent) { sink = append(sink, event) })
		result, err := machine.Execute(program)
		if err != nil {
			t.Fatal(err)
		}
		if got := result.Vars["evaluated"]; got.Kind != ValueInt || got.Int != 1 {
			t.Fatalf("message argument side effect = %#v, want 1", got)
		}
		if len(result.Debug) != 0 || len(result.DebugEvents) != 0 || len(sink) != 0 || stdout.Len() != 0 {
			t.Fatalf("NONE emitted output: debug=%#v events=%#v sink=%#v stdout=%q",
				result.Debug, result.DebugEvents, sink, stdout.String())
		}
	})
}

func assertDebugEventVector(t *testing.T, label string, got []DebugEvent, wantLevels, wantMessages []string) {
	t.Helper()
	if len(got) != len(wantLevels) {
		t.Fatalf("%s count = %d, want %d: %#v", label, len(got), len(wantLevels), got)
	}
	for i, event := range got {
		if event.Level != wantLevels[i] || event.Message != wantMessages[i] {
			t.Errorf("%s[%d] = (%q, %q), want (%q, %q)", label, i,
				event.Level, event.Message, wantLevels[i], wantMessages[i])
		}
	}
}
