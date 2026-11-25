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
			if !reflect.DeepEqual(calls, wantCalls) || (fault == "none" && !reflect.DeepEqual(values, want)) || (fault == "row" && rowErrors["first"] != "row failure") || (fault == "product" && (productErr == nil || productErr.Error() != "product failure")) || (fault == "browser" && (browserErr == nil || browserErr.Error() != "browser failure")) {
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
	if got["first"] != "parallel failure; serial retry: serial failure" || productErr != nil || browserErr != nil {
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
	// With five rows and four parallel pages, at least one page must drain another row.
	ids := []string{"a", "b", "c", "d", "e"}
	run := func(parallel bool, fault string) ([]byte, string) {
		t.Helper()
		config, err := json.Marshal(map[string]any{"url": "http://localhost/", "ids": ids, "playwrightModule": mock, "parallel": parallel})
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
	if !bytes.Equal(serial, parallel) || strings.Count(serialPages, "page\n") != 1 || strings.Count(parallelPages, "page\n") != 4 {
		t.Fatalf("success output or page count differs: serial=%s parallel=%s pages=%q/%q", serial, parallel, serialPages, parallelPages)
	}
	var success struct {
		Values map[string]json.RawMessage `json:"values"`
		Errors map[string]string          `json:"errors"`
	}
	if err := json.Unmarshal(parallel, &success); err != nil {
		t.Fatalf("decode successful observations: %v", err)
	}
	if len(success.Values) != len(ids) || len(success.Errors) != 0 {
		t.Fatalf("expected observations for %v: %s", ids, parallel)
	}
	for _, id := range ids {
		if _, ok := success.Values[id]; !ok {
			t.Fatalf("missing observation for %s: %s", id, parallel)
		}
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

func TestL12ObserverPreservesFirstFault(t *testing.T) {
	for _, fault := range []string{"row", "product", "browser"} {
		t.Run(fault, func(t *testing.T) {
			_, rows, product, browser := l12ObserveWithFallback(func(parallel bool) (map[string]json.RawMessage, map[string]string, error, error) {
				if !parallel {
					return nil, nil, nil, nil
				}
				switch fault {
				case "row":
					return nil, map[string]string{"first": "first row failure"}, nil, nil
				case "product":
					return nil, nil, errors.New("first product failure"), nil
				default:
					return nil, nil, nil, errors.New("first browser failure")
				}
			})
			if fault == "row" && rows["first"] != "first row failure" || fault == "product" && (product == nil || product.Error() != "first product failure") || fault == "browser" && (browser == nil || browser.Error() != "first browser failure") {
				t.Fatalf("first failure lost: rows=%v product=%v browser=%v", rows, product, browser)
			}
		})
	}
}

func TestL12ObserverReportsBothAttempts(t *testing.T) {
	want := map[string]json.RawMessage{"ok": json.RawMessage(`{"stage":"done"}`)}
	values, rows, product, browser := l12ObserveWithFallback(func(parallel bool) (map[string]json.RawMessage, map[string]string, error, error) {
		if parallel {
			return want, map[string]string{"first": "initial"}, errors.New("initial product"), errors.New("initial browser")
		}
		return nil, map[string]string{"second": "retry"}, errors.New("retry product"), errors.New("retry browser")
	})
	if !reflect.DeepEqual(values, want) || rows["first"] != "initial" || rows["second"] != "serial retry: retry" || product == nil || !strings.Contains(product.Error(), "initial product") || !strings.Contains(product.Error(), "retry product") || browser == nil || !strings.Contains(browser.Error(), "initial browser") || !strings.Contains(browser.Error(), "retry browser") {
		t.Fatalf("values=%v rows=%v product=%v browser=%v", values, rows, product, browser)
	}
}
