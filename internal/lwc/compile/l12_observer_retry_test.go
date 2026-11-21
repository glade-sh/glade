package compile_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestL12ObserverRetriesFaultsSerially(t *testing.T) {
	for _, fault := range []string{"none", "row", "product", "browser"} {
		t.Run(fault, func(t *testing.T) {
			var calls []bool
			want := map[string]json.RawMessage{"first": json.RawMessage(`{"stage":"done"}`)}
			values, rowErrors, productErr, browserErr := l12ObserveWithFallback(func(parallel bool) (map[string]json.RawMessage, map[string]string, error, error) {
				calls = append(calls, parallel)
				if parallel {
					switch fault {
					case "row":
						return nil, map[string]string{"first": "row failure"}, nil, nil
					case "product":
						return nil, nil, errors.New("product failure"), nil
					case "browser":
						return nil, nil, nil, errors.New("browser failure")
					}
				}
				return want, nil, nil, nil
			})
			wantCalls := []bool{true}
			if fault != "none" {
				wantCalls = append(wantCalls, false)
			}
			if !reflect.DeepEqual(calls, wantCalls) || !reflect.DeepEqual(values, want) || len(rowErrors) != 0 || productErr != nil || browserErr != nil {
				t.Fatalf("calls=%v values=%v rowErrors=%v productErr=%v browserErr=%v", calls, values, rowErrors, productErr, browserErr)
			}
		})
	}
}

func TestL12ObserverKeepsSerialFault(t *testing.T) {
	_, got, productErr, browserErr := l12ObserveWithFallback(func(parallel bool) (map[string]json.RawMessage, map[string]string, error, error) {
		if parallel {
			return nil, map[string]string{"first": "parallel failure"}, nil, nil
		}
		return nil, map[string]string{"first": "serial failure"}, nil, nil
	})
	if got["first"] != "serial failure" || productErr != nil || browserErr != nil {
		t.Fatalf("rowErrors=%v productErr=%v browserErr=%v", got, productErr, browserErr)
	}
}

func TestL12ObserverSchedulesAndReportsFaults(t *testing.T) {
	mock := filepath.Join(t.TempDir(), "playwright.cjs")
	const source = `module.exports.chromium = { launch: async () => ({
  newContext: async () => ({
    newPage: async () => {
      process.stderr.write('page\n');
      let id = '';
      return {
        on() {}, close: async () => {},
        goto: async url => { id = new URL(url).searchParams.get('c__case'); },
        evaluate: async () => process.env.L12_FAULT === 'startup' && id === 'b' ? {name:'Fault', message:'startup'} : null,
        locator: selector => selector.includes('data-run') ? {
          click: async () => { if (process.env.L12_FAULT === 'row' && id === 'b') throw Error('row fault'); }
        } : {
          waitFor: async () => {}, evaluate: async () => {},
          textContent: async () => JSON.stringify({id, stage:'done', trace:[]})
        }
      };
    }, close: async () => {}
  }), close: async () => {}
}) };`
	if err := os.WriteFile(mock, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(parallel bool, fault string) ([]byte, string) {
		t.Helper()
		config, err := json.Marshal(map[string]any{"url": "http://localhost/", "ids": []string{"a", "b", "c"}, "playwrightModule": mock, "parallel": parallel})
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("node", "testdata/l12_browser.mjs")
		cmd.Env = append(os.Environ(), "L12_FAULT="+fault)
		cmd.Stdin = bytes.NewReader(config)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("observer: %v: %s", err, stderr.String())
		}
		return out, stderr.String()
	}
	serial, serialPages := run(false, "")
	parallel, parallelPages := run(true, "")
	if !bytes.Equal(serial, parallel) || strings.Count(serialPages, "page\n") != 1 || strings.Count(parallelPages, "page\n") != 2 {
		t.Fatalf("success output or page count differs: serial=%s parallel=%s pages=%q/%q", serial, parallel, serialPages, parallelPages)
	}
	for _, fault := range []string{"startup", "row"} {
		for _, parallel := range []bool{false, true} {
			out, _ := run(parallel, fault)
			var result struct {
				Errors map[string]string `json:"errors"`
			}
			if err := json.Unmarshal(out, &result); err != nil || result.Errors["b"] == "" {
				t.Fatalf("fault=%s parallel=%v output=%s err=%v", fault, parallel, out, err)
			}
			if fault == "startup" && !parallel && result.Errors["c"] == "" {
				t.Fatalf("serial startup fault did not carry forward: %s", out)
			}
		}
	}
}
