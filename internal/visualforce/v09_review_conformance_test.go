package visualforce_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/visualforce"
)

type v09ReviewCase struct {
	ID           string                     `json:"id"`
	Kind         string                     `json:"kind"`
	Mode         string                     `json:"mode"`
	Inputs       map[string]v04Input        `json:"inputs"`
	Expected     map[string]json.RawMessage `json:"expected"`
	Parameters   map[string]string          `json:"parameters"`
	Transport    string                     `json:"transport"`
	URL          *string                    `json:"url"`
	PixelWidths  map[string]v09ReviewWidths `json:"pixel_widths"`
	MissingFacts map[string][]struct {
		Path   string `json:"path"`
		Reason string `json:"reason"`
	} `json:"missing_facts"`
}

type v09ReviewWidths struct {
	Paths  []string `json:"paths"`
	Reason string   `json:"reason"`
}

type v09ReviewTable struct {
	OracleCommit       string                `json:"oracle_commit"`
	SizingOracleCommit string                `json:"sizing_oracle_commit"`
	Fixtures           map[string]v09Fixture `json:"fixtures"`
	Cases              []v09ReviewCase       `json:"cases"`
}

// Native review controls distinguish attribute absence from empty text and
// observe the field's real browser actions, actual HTTP field pairs and action
// readback. Only browser-internal xhr/fetch categorization and hosted origins
// are outside the field projection. Recorded Choices pixel widths depend on
// the captured browser's font metrics; only those numeric values are qualified,
// while positive/equal/persistent widths and every other fact remain asserted.
// Native URL terminal navigation and body remain explicitly missing facts.
func v09RunReviewControls(t *testing.T, capture bool, writeRow func(...string)) {
	t.Helper()
	data, err := os.ReadFile("testdata/v09_review_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var table v09ReviewTable
	if err = json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.OracleCommit != "capture-revision-050121852438765532668128661008791653890373918807550628132304075192693615583240" ||
		table.SizingOracleCommit != "capture-revision-051206161228755014064858087442308836533411597852165854001629025759834915876999" || len(table.Cases) != 58 {
		t.Fatal("incomplete native V09 review controls")
	}
	total, rawTotal, qualifiedTotal := 0, 0, 0
	for _, api := range []string{"59.0", "67.0"} {
		root := t.TempDir()
		fixture := table.Fixtures[api]
		v09WriteFiles(t, root, fixture.Files)
		v09WriteFiles(t, root, map[string]string{fixture.ControllerPath: fixture.RuntimeController})
		// The API's one controller is compiled once. Page sources are small and
		// immutable; the HTTP server retains only the active row's request state.
		for _, row := range table.Cases {
			if row.Kind == "runtime" {
				v09WriteFiles(t, root, row.Inputs[api].Files)
			}
		}
		matches, rawMatches, qualifiedMatches, qualifiedRows := 0, 0, 0, 0
		compare := func(row v09ReviewCase, got string) {
			var expected any
			if err := json.Unmarshal(row.Expected[api], &expected); err != nil {
				t.Fatal(err)
			}
			want := v04Encode(t, expected)
			if row.Kind == "compile" {
				want = expected.(string)
			}
			rawGot, rawWant := got, want
			if rawGot == rawWant {
				rawMatches++
			}
			widths, recorded := row.PixelWidths[api]
			if recorded && (row.Kind != "runtime" || row.Mode != "choices" || widths.Reason == "" || len(widths.Paths) == 0) {
				t.Fatalf("%s: invalid recorded pixel-width qualification at API %s", row.ID, api)
			}
			// The native widths come from Mac Chrome, so macOS asserts them exactly.
			// Other hosts have different font metrics: only there are the recorded
			// paths qualified to a positive pixel width, with raw values reported.
			qualified := recorded && runtime.GOOS != "darwin"
			if qualified {
				widths.Reason += "; host " + runtime.GOOS + " is not the oracle host family (darwin)"
				qualifiedRows++
				got, want = v09QualifyReviewWidths(t, got, expected, widths.Paths)
				t.Logf("V09 review API %s %s pixel-width qualification: %s", api, row.ID, widths.Reason)
			}
			status := "MISMATCH"
			if got == want {
				status = "MATCH"
				matches++
				if qualified {
					qualifiedMatches++
				}
			} else if !capture {
				t.Errorf("V09 review API %s %s actual <%s> expected <%s>", api, row.ID, got, want)
			}
			// Reports retain the raw native and local answers even when the
			// comparison has a recorded, reviewed boundary.
			qualification := ""
			if qualified {
				qualification = widths.Reason
			}
			writeRow(api, row.ID, status, rawGot, rawWant, "review-controls", "", "", qualification)
			for _, fact := range row.MissingFacts[api] {
				t.Logf("V09 review API %s %s missing native fact %s: %s", api, row.ID, fact.Path, fact.Reason)
			}
		}
		for _, row := range table.Cases {
			if row.Kind != "compile" {
				continue
			}
			answer, diagnostics := v04Execute(t, root, row.Inputs[api].Name, v04Case{Kind: "compile"})
			if len(diagnostics) != 0 {
				answer += "|" + v04Encode(t, diagnostics)
			}
			compare(row, answer)
			v09ReleaseRowState(t, root)
		}
		v09ReviewBrowser(t, root, api, table.Cases, compare)
		visualforce.V09ReleaseRenderEnvironmentsForTest(root)
		v09ReleaseRowState(t, root)
		t.Logf("V09 review API %s matched %d/%d; raw exact %d/%d; pixel-width qualified %d/%d", api, matches, len(table.Cases), rawMatches, len(table.Cases), qualifiedMatches, qualifiedRows)
		total += matches
		rawTotal += rawMatches
		qualifiedTotal += qualifiedRows
	}
	writeRow("REVIEW_TOTAL", fmt.Sprintf("%d/%d", total, 2*len(table.Cases)))
	writeRow("REVIEW_RAW_EXACT_TOTAL", fmt.Sprintf("%d/%d", rawTotal, 2*len(table.Cases)))
	t.Logf("V09 review controls matched %d/%d; raw exact %d/%d; %d rows have recorded pixel-width qualifications", total, 2*len(table.Cases), rawTotal, 2*len(table.Cases), qualifiedTotal)
}

var v09ReviewPixelStyle = regexp.MustCompile(`^width: ([0-9]+(?:\.[0-9]+)?)px;$`)

func v09PositivePixelStyle(style any) bool {
	text, ok := style.(string)
	if !ok {
		return false
	}
	parts := v09ReviewPixelStyle.FindStringSubmatch(text)
	if len(parts) != 2 {
		return false
	}
	width, err := strconv.ParseFloat(parts[1], 64)
	return err == nil && width > 0
}

// Each path is exported from a recorded width declaration on a visible select.
// Null/no-op styles, the hidden backing select, and other CSS stay byte-exact.
func v09ReviewWidthParent(value any, path string) (map[string]any, bool) {
	if !strings.HasPrefix(path, "/") ||
		!(strings.HasSuffix(path, "/selects/1/style") || strings.HasSuffix(path, "/selects/2/style")) {
		return nil, false
	}
	parts := strings.Split(path[1:], "/")
	for _, part := range parts[:len(parts)-1] {
		switch node := value.(type) {
		case map[string]any:
			value = node[part]
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(node) {
				return nil, false
			}
			value = node[index]
		default:
			return nil, false
		}
	}
	parent, ok := value.(map[string]any)
	return parent, ok && parent["visible"] == true && parent["multiple"] == true && parent["type"] == "select-multiple"
}

func v09QualifyReviewWidths(t *testing.T, got string, expected any, paths []string) (string, string) {
	t.Helper()
	want := v04Encode(t, expected)
	var actual any
	if json.Unmarshal([]byte(got), &actual) != nil {
		return got, want
	}
	var nativeStyle, localStyle any
	parents := make([]map[string]any, 0, 2*len(paths))
	for _, path := range paths {
		native, ok := v09ReviewWidthParent(expected, path)
		if !ok || !v09PositivePixelStyle(native["style"]) || (nativeStyle != nil && nativeStyle != native["style"]) {
			t.Fatalf("invalid native Choices width qualification %s", path)
		}
		nativeStyle = native["style"]
		local, ok := v09ReviewWidthParent(actual, path)
		if !ok || !v09PositivePixelStyle(local["style"]) || (localStyle != nil && localStyle != local["style"]) {
			// Unequal lists, changing widths, absent declarations, extra CSS,
			// invalid units and zero widths are ordinary conformance failures.
			return got, want
		}
		localStyle = local["style"]
		parents = append(parents, native, local)
	}
	for _, parent := range parents {
		parent["style"] = "width: <browser-pixels>;"
	}
	return v04Encode(t, actual), v04Encode(t, expected)
}

func v09ReviewBrowser(t *testing.T, root, api string, cases []v09ReviewCase, compare func(v09ReviewCase, string)) {
	t.Helper()
	renderer := v09NewRenderer(t, root)
	defer visualforce.V09ReleaseRenderEnvironmentsForTest(root)
	byID := map[string]v09ReviewCase{}
	specs := []map[string]any{}
	for _, row := range cases {
		if row.Kind != "runtime" {
			continue
		}
		byID[row.ID] = row
		specs = append(specs, map[string]any{"id": row.ID, "path": "/apex/" + row.Inputs[api].Name, "mode": row.Mode, "transport": row.Transport, "parameters": row.Parameters, "url": row.URL})
	}
	rowReleased := make(chan struct{})
	var mu sync.Mutex
	var current *server.Server
	var currentPath string
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__v09_next" {
			select {
			case <-rowReleased:
				w.WriteHeader(http.StatusNoContent)
			case <-r.Context().Done():
			}
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if strings.HasPrefix(r.URL.Path, "/apex/") && (current == nil || currentPath != r.URL.Path) {
			org := renderer.org.CloneRuntimeOrg()
			org.APIVersion = api
			current = server.NewWithSource(&org, renderer.source)
			current.VisualforceHTMLUserID = renderer.principal
			machine := renderer.runtime.CloneRuntime(nil)
			for _, row := range cases {
				if row.Kind == "runtime" {
					machine.RegisterPageReference(row.Inputs[api].Name)
				}
			}
			current.SetProjectRuntime(renderer.index, machine, renderer.runtimeErr)
			currentPath = r.URL.Path
		}
		if current == nil {
			http.NotFound(w, r)
			return
		}
		current.ServeHTTP(w, r)
	}))
	defer local.Close()
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	module := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if module == "" {
		module = filepath.Join(repo, "lwcruntime/node_modules/playwright")
	}
	config, err := json.Marshal(map[string]any{"url": local.URL, "specs": specs, "playwrightModule": module, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), "roles": map[string]string{"ownerName": v09OwnerName, "ownerId": string(renderer.principal), "accountId": v09AccountID}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "-e", v09ReviewObserver)
	cmd.Stdin = bytes.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	for scanner.Scan() {
		var observation struct {
			ID    string          `json:"id"`
			Value json.RawMessage `json:"value"`
		}
		if err = json.Unmarshal(scanner.Bytes(), &observation); err != nil {
			cancel()
			_ = cmd.Wait()
			t.Fatal(err)
		}
		row, exists := byID[observation.ID]
		if !exists || seen[row.ID] {
			cancel()
			_ = cmd.Wait()
			t.Fatalf("invalid V09 review browser row %q", observation.ID)
		}
		seen[row.ID] = true
		var value any
		if err = json.Unmarshal(observation.Value, &value); err != nil {
			cancel()
			_ = cmd.Wait()
			t.Fatal(err)
		}
		compare(row, v04Encode(t, value))
		// The browser has returned to about:blank before publishing this row.
		// Release the sole page handler and its request VM before the next row.
		mu.Lock()
		current = nil
		currentPath = ""
		mu.Unlock()
		v09ReleaseRowState(t, root)
		select {
		case rowReleased <- struct{}{}:
		case <-ctx.Done():
			cancel()
			_ = cmd.Wait()
			t.Fatal(ctx.Err())
		}
	}
	if err = scanner.Err(); err != nil {
		cancel()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	if err = cmd.Wait(); err != nil {
		t.Fatalf("V09 review API %s browser failed: %v: %s", api, err, stderr.String())
	}
	if len(seen) != len(byID) {
		t.Fatalf("V09 review API %s browser returned %d/%d rows", api, len(seen), len(byID))
	}
}

// Observer operations are copied from the captured native program; fixtures and
// expected answers never reach the browser. Output is acknowledged one row at a
// time, so no page, controller or rendered response survives the next row.
const v09ReviewObserver = `// Native review controls. The runner prepends NODE_PAGE_SHARDS.
// Only field-owned data crosses stdout; authenticated URLs and request bodies
// remain private. No vendor function is replaced or invoked by this observer.
const fs = require('node:fs');

function byID(id) { return '[id=' + JSON.stringify(id) + ']'; }
function rowSelector(id) { return '[data-v09-case=' + JSON.stringify(id) + ']'; }

async function reviewFrame(page, id, timeout = 10000) {
  const deadline = Date.now() + timeout;
  do {
    for (const frame of page.frames()) {
      try {
        const observed = await frame.evaluate(id => {
          const matches = [...document.querySelectorAll('[data-v09-case]')].filter(node => node.getAttribute('data-v09-case') === id);
          const row = matches[0];
          const review = row && (row.matches('[data-v09-review-row]') || row.querySelector('[data-v09-review-row]') || row.closest('[data-v09-review-row]'));
          return matches.length === 1 && !!review && !!document.querySelector('[data-v09-control-context]');
        }, id);
        if (observed) return frame;
      } catch (_) {} // A native login/frame redirect can replace a frame.
    }
    if (Date.now() >= deadline) break;
    await new Promise(resolve => setTimeout(resolve, 100));
  } while (true);
  throw new Error('NATIVE_REVIEW_MARKER_MISSING');
}

function remaining(deadline, maximum = 10000) {
  return Math.max(1, Math.min(maximum, deadline - Date.now()));
}

async function lookupFacts(page, id, roles) {
  const surface = await reviewFrame(page, id);
  return surface.evaluate(({id, roles}) => {
    const row = [...document.querySelectorAll('[data-v09-case]')].find(node => node.getAttribute('data-v09-case') === id);
    const inputs = [...row.querySelectorAll('input:not([type="hidden"])')].filter(node => /(^|:)owned$/.test(node.id));
    if (inputs.length !== 1) throw new Error('LOOKUP_OWNED_INPUT_MISSING');
    const input = inputs[0];
    const form = input.form;
    if (!form || !row.contains(form)) throw new Error('LOOKUP_OWNED_FORM_MISSING');
    const stable = value => {
      if (value === null) return null;
      let text = String(value);
      for (const [raw, token] of [[roles.ownerName, 'OWNER_NAME'], [roles.accountId, 'OWNED_ACCOUNT_ID'], [roles.ownerId, 'OWNER_ID']]) {
        if (!raw) continue;
        text = text.split(raw).join(token);
        if (token !== 'OWNER_NAME') text = text.split(raw.slice(0, 15)).join(token);
      }
      return text;
    };
    const suffixes = ['_lkid', '_lkold', '_lktp', '_lspf', '_lspfsub', '_mod'];
    const hidden = [...form.querySelectorAll('input[type="hidden"]')].filter(node => suffixes.some(suffix =>
      node.id === input.id + suffix || node.name === input.id + suffix));
    const field = node => ({tag: node.tagName.toLowerCase(), id: node.getAttribute('id'), name: node.getAttribute('name'),
      type: node.getAttribute('type'), valueAttribute: stable(node.getAttribute('value')), value: stable(node.value),
      disabled: node.disabled, required: node.required, class: node.getAttribute('class'), style: node.getAttribute('style'),
      tabindex: node.getAttribute('tabindex'), size: node.getAttribute('size'), maxlength: node.getAttribute('maxlength')});
    return {input: field(input), form: {id: form.getAttribute('id'), name: form.getAttribute('name'), method: form.getAttribute('method')},
      hidden: hidden.map((node, order) => ({order, ...field(node)})), hiddenCount: hidden.length,
      suffixCounts: suffixes.map(suffix => ({suffix, count: hidden.filter(node => node.id === input.id + suffix || node.name === input.id + suffix).length})),
      associatedOrder: [...form.querySelectorAll('input')].filter(node => node === input || hidden.includes(node)).map(node => node.getAttribute('id'))};
  }, {id, roles});
}

async function choicesFacts(page, id) {
  const surface = await reviewFrame(page, id);
  return surface.evaluate(id => {
    const row = [...document.querySelectorAll('[data-v09-case]')].find(node => node.getAttribute('data-v09-case') === id);
    const controls = [...row.querySelectorAll('select')];
    const backing = controls.filter(node => /(^|:)owned$/.test(node.id));
    if (backing.length !== 1 || !backing[0].form) throw new Error('CHOICES_BACKING_SELECT_MISSING');
    const field = backing[0];
    const selects = controls.filter(node => node.id === field.id || node.id === field.id + '_unselected' || node.id === field.id + '_selected');
    const options = node => [...node.options].map(option => ({text: option.text, value: option.value, valueAttribute: option.getAttribute('value'),
      selected: option.selected, defaultSelected: option.defaultSelected, disabled: option.disabled}));
    const links = [...row.querySelectorAll('a[href]')].filter(link => [...link.querySelectorAll('img')].some(img =>
      img.id === field.id + '_right_arrow' || img.id === field.id + '_left_arrow')).map(link => ({href: link.getAttribute('href'),
        images: [...link.querySelectorAll('img')].map(img => ({id: img.getAttribute('id'), class: img.getAttribute('class'),
          alt: img.getAttribute('alt'), title: img.getAttribute('title')}))}));
    return {backingID: field.id, fieldName: field.name, selectCount: selects.length,
      selects: selects.map(node => ({id: node.getAttribute('id'), name: node.getAttribute('name'), type: node.type,
        style: node.getAttribute('style'), class: node.getAttribute('class'), multiple: node.multiple, disabled: node.disabled,
        visible: !!(node.getClientRects().length && getComputedStyle(node).visibility !== 'hidden'), value: node.value, options: options(node)})),
      links, formEntries: [...new FormData(field.form).entries()].filter(([name]) => name === field.name).map(([name, value]) => [name, String(value)])};
  }, id);
}

function optionShape(facts) {
  return JSON.stringify(facts.selects.map(select => ({id: select.id, options: select.options.map(option =>
    ({text: option.text, value: option.value, selected: option.selected}))})));
}

async function arrowStep(page, id, facts, direction, chooseLast, deadline) {
  if (Date.now() >= deadline) return {direction, status: 'unobserved', reason: 'ROW_TIME_BUDGET_EXHAUSTED', after: facts};
  const surface = await reviewFrame(page, id, remaining(deadline));
  facts = await choicesFacts(page, id);
  const sourceID = facts.backingID + (direction === 'right' ? '_unselected' : '_selected');
  const source = facts.selects.find(select => select.id === sourceID);
  if (!source) return {direction, status: 'unobserved', reason: 'NATIVE_VISIBLE_SELECT_MISSING', after: facts};
  const options = source.options.map((option, index) => ({option, index})).filter(({option}) => !option.disabled);
  if (!options.length) return {direction, status: 'not-applicable', reason: 'NATIVE_SOURCE_HAS_NO_SELECTABLE_OPTIONS', after: facts};
  const chosen = chooseLast ? options.slice(-1) : options;
  await surface.locator(byID(sourceID)).selectOption(chosen.map(({index}) => ({index})), {timeout: remaining(deadline, 5000)});
  const selected = await choicesFacts(page, id);
  const arrowID = facts.backingID + (direction === 'right' ? '_right_arrow' : '_left_arrow');
  const arrow = surface.locator(rowSelector(id)).locator('a').filter({has: surface.locator(byID(arrowID))});
  if (await arrow.count() !== 1) return {direction, status: 'unobserved', reason: 'NATIVE_ARROW_LINK_MISSING', selected, after: selected};
  let clickReason = null;
  try { await arrow.click({timeout: remaining(deadline, 5000), noWaitAfter: true}); } catch (_) { clickReason = 'NATIVE_ARROW_CLICK_FAILED'; }
  let changed = false;
  if (!clickReason) {
    try {
      await surface.waitForFunction(({id, before}) => {
        const row = [...document.querySelectorAll('[data-v09-case]')].find(node => node.getAttribute('data-v09-case') === id);
        if (!row) return false;
        const backing = [...row.querySelectorAll('select')].find(node => /(^|:)owned$/.test(node.id));
        if (!backing) return false;
        const selects = [...row.querySelectorAll('select')].filter(node => [backing.id, backing.id + '_unselected', backing.id + '_selected'].includes(node.id));
        const now = JSON.stringify(selects.map(select => ({id: select.getAttribute('id'), options: [...select.options].map(option =>
          ({text: option.text, value: option.value, selected: option.selected}))})));
        return now !== before;
      }, {id, before: optionShape(selected)}, {timeout: remaining(deadline, 4000)});
      changed = true;
    } catch (_) { clickReason = 'NATIVE_ARROW_NO_DOM_CHANGE'; }
  }
  return {direction, status: changed ? 'observed' : 'unobserved', reason: clickReason, selected,
    after: await choicesFacts(page, id)};
}

async function submitChoices(page, id, spec, facts, deadline) {
  if (Date.now() >= deadline) return {result: null, requests: [], reason: 'ROW_TIME_BUDGET_EXHAUSTED'};
  const surface = await reviewFrame(page, id, remaining(deadline));
  const sourceURL = new URL(surface.url());
  const requests = [];
  const listener = request => {
    try {
      const target = new URL(request.url());
      if (request.method() !== 'POST' || target.origin !== sourceURL.origin || target.pathname.toLowerCase() !== sourceURL.pathname.toLowerCase()) return;
      const contentType = request.headers()['content-type'] || '';
      if (!contentType.toLowerCase().startsWith('application/x-www-form-urlencoded')) {
        requests.push({method: request.method(), resourceType: request.resourceType(), fieldName: facts.fieldName,
          entries: null, reason: 'NATIVE_SUBMISSION_BODY_ENCODING_UNOBSERVED'});
        return;
      }
      // Parse privately and retain only the exact field-name projection. Never
      // keep the entire request, unrelated parameter names, or ViewState/CSRF.
      const body = request.postData();
      const entries = body === null ? null : [...new URLSearchParams(body).entries()].filter(([name]) => name === facts.fieldName);
      requests.push({method: request.method(), resourceType: request.resourceType(), fieldName: facts.fieldName, entries,
        reason: body === null ? 'NATIVE_REQUEST_BODY_UNAVAILABLE' : null});
    } catch (_) {
      requests.push({reason: 'NATIVE_SUBMISSION_PROJECTION_FAILED'});
    }
  };
  const selector = rowSelector(id) + ' [data-v09-review-result]';
  const initialResult = await surface.locator(selector).textContent();
  let navigation = null;
  let reason = null;
  page.on('request', listener);
  try {
    // The page contains one owned commandButton. Match its explicit component
    // ID and label rather than replacing its native submit or AJAX handler.
    const submit = surface.locator(rowSelector(id)).locator('[id$=":reviewSubmit"], [id="reviewSubmit"]');
    if (await submit.count() !== 1 || await submit.getAttribute('value') !== 'V09 submit') throw new Error('NATIVE_SUBMIT_BUTTON_MISSING');
    if (spec.transport === 'ajax') {
      await submit.click({timeout: remaining(deadline, 5000)});
    } else {
      const observedNavigation = surface.waitForNavigation({waitUntil: 'domcontentloaded', timeout: remaining(deadline, 15000)}).then(response =>
        ({observed: true, status: response ? response.status() : null}), () => ({observed: false, reason: 'NATIVE_POST_NAVIGATION_NOT_OBSERVED'}));
      await submit.click({timeout: remaining(deadline, 5000), noWaitAfter: true});
      navigation = await observedNavigation;
    }
    try {
      await surface.waitForFunction(({id, initialResult}) => {
        const row = [...document.querySelectorAll('[data-v09-case]')].find(node => node.getAttribute('data-v09-case') === id);
        const result = row && row.querySelector('[data-v09-review-result]');
        return result && result.textContent !== initialResult;
      }, {id, initialResult}, {timeout: remaining(deadline, 15000)});
    } catch (_) { reason = 'NATIVE_ACTION_RESULT_NOT_OBSERVED'; }
    const resultSurface = await reviewFrame(page, id, remaining(deadline));
    if (!requests.length && !reason) reason = 'NATIVE_SUBMISSION_REQUEST_NOT_OBSERVED';
    return {initialResult, result: await resultSurface.locator(selector).textContent(), requests, navigation, reason};
  } catch (_) {
    return {initialResult, result: null, requests, navigation, reason: 'NATIVE_SUBMISSION_NOT_OBSERVED'};
  } finally {
    page.off('request', listener);
  }
}

async function choicesCapture(page, id, spec, url) {
  const deadline = Date.now() + 60000;
  const initial = await choicesFacts(page, id);
  const capture = {initial, steps: {}, emptySubmission: null, selectedTwin: null, untouched: null, errors: []};
  if (initial.selectCount !== 3) {
    capture.errors.push('NATIVE_CHOICES_THREE_SELECTS_MISSING');
    return capture;
  }
  const step = async (facts, direction, last) => {
    try { return await arrowStep(page, id, facts, direction, last, deadline); }
    catch (_) {
      const result = {direction, status: 'unobserved', reason: 'NATIVE_ARROW_OPERATION_FAILED', after: facts};
      try { result.after = await choicesFacts(page, id); } catch (_) { result.afterUnobserved = true; }
      return result;
    }
  };
  capture.steps.addAll = await step(initial, 'right', false);
  capture.steps.removeAll = await step(capture.steps.addAll.after, 'left', false);
  try { capture.emptySubmission = await submitChoices(page, id, spec, capture.steps.removeAll.after, deadline); }
  catch (_) { capture.errors.push('EMPTY_SUBMISSION_UNOBSERVED'); }
  try {
    if (Date.now() >= deadline) throw new Error('ROW_TIME_BUDGET_EXHAUSTED');
    await page.goto(url, {waitUntil: 'domcontentloaded', timeout: remaining(deadline, 15000)});
    const selectedInitial = await choicesFacts(page, id);
    capture.selectedTwin = {initial: selectedInitial};
    const removeAll = await step(selectedInitial, 'left', false);
    capture.selectedTwin.removeAll = removeAll;
    const addLast = await step(removeAll.after, 'right', true);
    capture.selectedTwin.addLast = addLast;
    capture.selectedTwin.submission = await submitChoices(page, id, spec, addLast.after, deadline);
  } catch (_) { capture.errors.push('SELECTED_SUBMISSION_UNOBSERVED'); }
  try {
    if (Date.now() >= deadline) throw new Error('ROW_TIME_BUDGET_EXHAUSTED');
    await page.goto(url, {waitUntil: 'domcontentloaded', timeout: remaining(deadline, 15000)});
    const untouchedInitial = await choicesFacts(page, id);
    capture.untouched = {initial: untouchedInitial};
    Object.assign(capture.untouched, await submitChoices(page, id, spec, untouchedInitial, deadline));
  } catch (_) { capture.errors.push('UNTOUCHED_SUBMISSION_UNOBSERVED'); }
  return capture;
}


function reviewURL(raw, base) {
  if(raw===null)return null;
  const parsed=new URL(raw,base);
  const decode=value=>decodeURIComponent(value.replace(/\+/g,' '));
  return {pathname:parsed.pathname,query:parsed.search?parsed.search.slice(1).split('&').map(pair=>{
    const at=pair.indexOf('=');const key=at<0?pair:pair.slice(0,at);const value=at<0?'':pair.slice(at+1);
    return {key:decode(key),encodedKey:key,hasEquals:at>=0,value:decode(value),encodedValue:value,reason:null};
  }):[]};
}

async function reviewURLCapture(page, context, id, localURL, scriptErrors) {
 const surface=await reviewFrame(page,id);
 const observed=await surface.locator(rowSelector(id)).evaluate(row=>{
  const copy=row.cloneNode(true);copy.querySelectorAll('script,style').forEach(n=>n.remove());
  return {text:copy.textContent,anchors:[...row.querySelectorAll('a[href]')].map(anchor=>({href:anchor.getAttribute('href'),text:(()=>{const c=anchor.cloneNode(true);c.querySelectorAll('script,style').forEach(n=>n.remove());return c.textContent;})(),target:anchor.getAttribute('target'),rel:anchor.getAttribute('rel')}))};
 });
 const result={text:observed.text,anchors:observed.anchors.map(a=>({...a,href:reviewURL(a.href,surface.url())}))};
 if(!observed.anchors.length){result.click={status:'not-applicable',reason:'NATIVE_ANCHOR_ABSENT'};return result;}
 const start=page.url(), existing=new Set(context.pages());
 const click={navigationObserved:false,newPage:false,sourcePageChanged:false,dialogs:[],errors:scriptErrors,requests:[],responses:[]};
 const owned=url=>{try{const v=new URL(url);return v.origin===new URL(localURL).origin&&v.pathname==='/servlet/servlet.ExternalRedirect';}catch(_){return false;}};
 const request=r=>{if(owned(r.url()))click.requests.push({method:r.method(),url:reviewURL(r.url(),localURL)});};
 const response=r=>{if(owned(r.url())){const location=r.headers()['location'];click.responses.push({status:r.status(),url:reviewURL(r.url(),localURL),location:location?reviewURL(location,r.url()):null});}};
 const dialog=d=>{click.dialogs.push(d.type());d.dismiss().catch(()=>{});};
 context.on('request',request);context.on('response',response);page.on('dialog',dialog);
 try {
  const popupPromise=page.waitForEvent('popup',{timeout:8000}).catch(()=>null);
  await surface.locator(rowSelector(id)+' a[href]').first().click({noWaitAfter:true});
  const popup=await popupPromise;
  if(popup){click.newPage=true;await popup.waitForLoadState('domcontentloaded',{timeout:8000});}
  click.navigationObserved=click.requests.length>0;
  click.sourcePageChanged=page.url()!==start;
  result.click=click;
  return result;
 } finally {
  context.off('request',request);context.off('response',response);page.off('dialog',dialog);
  for(const popup of context.pages())if(!existing.has(popup))await popup.close();
 }
}

function reviewFieldProjection(value) {
 if(Array.isArray(value))return value.map(reviewFieldProjection);
 if(value&&typeof value==='object')return Object.fromEntries(Object.entries(value).filter(([key])=>key!=='resourceType').map(([key,v])=>[key,reviewFieldProjection(v)]));
 return value;
}

(async()=>{
 const cfg=JSON.parse(fs.readFileSync(0,'utf8'));
 const {chromium}=require(cfg.playwrightModule);
 const browser=await chromium.launch({headless:true,...(cfg.executablePath?{executablePath:cfg.executablePath}:{})});
 const context=await browser.newContext({locale:'en-US',timezoneId:'UTC',serviceWorkers:'block'});
 const page=await context.newPage();
 try {
  await context.route('**/*',route=>{
   const target=new URL(route.request().url());
   if(target.origin===new URL(cfg.url).origin||target.protocol==='data:'||target.protocol==='about:')return route.continue();
   return route.abort();
  });
  for(const spec of cfg.specs){
   // Install before navigation: inline onload failures occur before the field
   // observer starts. New popup pages join the same bounded row listener set.
   const scriptErrors=[];
   const watchedPages=new Map();
   const watchPage=observedPage=>{
    if(watchedPages.has(observedPage))return;
    const onError=()=>scriptErrors.push('NATIVE_SCRIPT_ERROR');
    watchedPages.set(observedPage,onError);observedPage.on('pageerror',onError);
   };
   watchPage(page);context.on('page',watchPage);
   const target=new URL(spec.path,cfg.url);target.searchParams.set('ownedCase',spec.id);
   for(const [key,value] of Object.entries(spec.parameters||{}))target.searchParams.set(key,value);
   if(spec.url!==null&&spec.url!==undefined)target.searchParams.set('ownedURL',spec.url);
   let value;
   try {
    await page.goto(target.href,{waitUntil:'domcontentloaded',timeout:30000});
    if(spec.mode==='lookup'){
     const first=await lookupFacts(page,spec.id,cfg.roles);
     await page.goto(target.href,{waitUntil:'domcontentloaded',timeout:30000});
     const repeat=await lookupFacts(page,spec.id,cfg.roles);
     value={first,repeat,stable:JSON.stringify(first)===JSON.stringify(repeat)};
    }else if(spec.mode==='choices')value=reviewFieldProjection(await choicesCapture(page,spec.id,spec,target.href));
    else value=await reviewURLCapture(page,context,spec.id,cfg.url,scriptErrors);
   }catch(error){value={localError:String(error.message)};}
   try {await page.goto('about:blank');}
   finally {
    context.off('page',watchPage);
    for(const [observedPage,onError] of watchedPages)observedPage.off('pageerror',onError);
   }
   if(scriptErrors.length){
    if(spec.mode==='choices'&&Array.isArray(value.errors))value.errors.push(...scriptErrors);
    else if(spec.mode==='url'&&value.click)value.click.errors=scriptErrors;
    else value.localScriptErrors=scriptErrors;
   }
   process.stdout.write(JSON.stringify({id:spec.id,value})+'\n');
   await fetch(cfg.url+'/__v09_next');
  }
 }finally{await page.close();await context.close();await browser.close();}
})().catch(error=>{process.stderr.write(String(error.stack||error));process.exitCode=1;});
`
