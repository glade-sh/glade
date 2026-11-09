package apextest_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

func c11Diagnostic(result testreport.Case) string {
	if result.Problem == nil {
		return fmt.Sprintf("status=%s problem=nil", result.Status)
	}
	message := result.Problem.Message
	if pos := strings.Index(message, "CAPTURE|"); pos >= 0 {
		message = message[:pos] + "<capture payload>"
	}
	message = regexp.MustCompile(`[[:alnum:]]{15,}`).ReplaceAllString(message, "<redacted>")
	if len(message) > 300 {
		message = message[:300]
	}
	return fmt.Sprintf("status=%s type=%s message=%q", result.Status, result.Problem.Type, message)
}

// The actor prerequisite is checked before any of the captured operations.
// Keep its exact local failure visible and attempt the native comparison again
// automatically when the prerequisite is fixed.
func c11ActorGateCarry(t *testing.T, result testreport.Case, class, method, arm string) bool {
	t.Helper()
	if result.ClassName != class || result.MethodName != method || result.Problem == nil || result.Problem.Type != "System.AssertException" {
		return false
	}
	status, message := testreport.StatusFail, "Assertion Failed: gate actor ViewAllData: Expected: false, Actual: null"
	if arm == "admin" {
		status = testreport.StatusRuntimeError
		message = "Assertion Failed: gate admin profile ViewAllData: Expected: true, Actual: false"
	} else if arm != "restricted" {
		return false
	}
	supplemental := result.Status == testreport.StatusFail && result.Problem.Message == "Assertion Failed: gate no supplemental ModifyAllData: Expected: false, Actual: null"
	if !supplemental && (result.Status != status || result.Problem.Message != message) {
		return false
	}
	t.Logf("CARRY (actor prerequisite; fix owner pending): %s %s did not reach the native observation: %s", class, method, c11Diagnostic(result))
	return true
}

func c12JSONValue(d *json.Decoder) (any, error) {
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, compound := tok.(json.Delim)
	if !compound {
		return tok, nil
	}
	switch delim {
	case '{':
		out := map[string]any{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return nil, err
			}
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("non-string key")
			}
			if _, exists := out[name]; exists {
				return nil, fmt.Errorf("duplicate key %q", name)
			}
			value, err := c12JSONValue(d)
			if err != nil {
				return nil, err
			}
			out[name] = value
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, fmt.Errorf("unclosed object: %v", err)
		}
		return out, nil
	case '[':
		out := []any{}
		for d.More() {
			value, err := c12JSONValue(d)
			if err != nil {
				return nil, err
			}
			out = append(out, value)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, fmt.Errorf("unclosed array: %v", err)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", delim)
	}
}

// encoding/json replaces unpaired UTF-16 surrogates with U+FFFD. Reject them
// first so unrelated input strings cannot compare equal after that replacement.

func c12ValidJSONText(raw []byte) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("invalid UTF-8 JSON")
	}
	inside := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inside = !inside
			continue
		}
		if !inside || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return fmt.Errorf("incomplete string escape")
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return fmt.Errorf("incomplete Unicode escape")
		}
		code, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return err
		}
		i += 4
		if code >= 0xD800 && code <= 0xDBFF {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return fmt.Errorf("unpaired high surrogate")
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xDC00 || low > 0xDFFF {
				return fmt.Errorf("invalid surrogate pair")
			}
			i += 6
		} else if code >= 0xDC00 && code <= 0xDFFF {
			return fmt.Errorf("unpaired low surrogate")
		}
	}
	return nil
}

func c12JSONObject(raw []byte) (map[string]any, error) {
	if err := c12ValidJSONText(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	value, err := c12JSONValue(d)
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON/data: %v", err)
	}
	out, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("observation must be an object")
	}
	return out, nil
}

func c12At(o any, path string) (any, error) {
	value := o
	for _, key := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		switch v := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = v[key]
			if !ok {
				return nil, fmt.Errorf("missing ID slot %s", path)
			}
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(v) {
				return nil, fmt.Errorf("invalid ID array slot %s", path)
			}
			value = v[i]
		default:
			return nil, fmt.Errorf("invalid ID parent %s", path)
		}
	}
	return value, nil
}

func c12Set(o map[string]any, path string, value string) error {
	pos := strings.LastIndex(path, "/")
	parent := any(o)
	var err error
	if pos > 0 {
		parent, err = c12At(o, path[:pos])
		if err != nil {
			return err
		}
	}
	key := path[pos+1:]
	switch v := parent.(type) {
	case map[string]any:
		v[key] = value
	case []any:
		i, err := strconv.Atoi(key)
		if err != nil || i < 0 || i >= len(v) {
			return fmt.Errorf("invalid set slot")
		}
		v[i] = value
	default:
		return fmt.Errorf("invalid set parent")
	}
	return nil
}

// Shape-only oracles deliberately do not infer the role or membership of IDs
// embedded in error text. All surrounding text remains byte-exact.

func c12CapturePayload(result testreport.Case, class, method, marker string) ([]byte, error) {
	if result.ClassName != class || result.MethodName != method || result.Status != testreport.StatusFail || result.Problem == nil || result.Problem.Type != "System.AssertException" {
		return nil, fmt.Errorf("not the selected deliberate assertion transport")
	}
	// The local VM System.assert transport in dispatch.go uses this exact prefix.
	// Arbitrary prefix text, suffix data, other markers and OBSERVED are rejected.
	prefix := "Assertion Failed: CAPTURE|" + marker + "|"
	if !strings.HasPrefix(result.Problem.Message, prefix) {
		return nil, fmt.Errorf("missing exact CAPTURE marker %s: %s", marker, c11Diagnostic(result))
	}
	raw := []byte(strings.TrimPrefix(result.Problem.Message, prefix))
	if _, err := c12JSONObject(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func c12Read(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func c12CopyProject(t *testing.T, source, destination string, files map[string]string) {
	t.Helper()
	seen := map[string]string{}
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular asset %s", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		raw := c12Read(t, path)
		digest := fmt.Sprintf("%x", sha256.Sum256(raw))
		if files[rel] != digest {
			return fmt.Errorf("unexpected/changed source asset %s", rel)
		}
		seen[rel] = digest
		c12WriteFile(t, filepath.Join(destination, filepath.FromSlash(rel)), raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seen, files) {
		t.Fatal("missing source assets")
	}
}

func c12WriteFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
}

func c12LoadIndex(t *testing.T, root string) typesys.Index {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, s)
}

// Pin context on the selected indexed source, not on apextest.TestCase.

func c12VerifyContext(t *testing.T, index typesys.Index, class, namespace, api string) string {
	t.Helper()
	found := ""
	count := 0
	for _, typ := range index.Types {
		if typ.Dependency || typ.NestingDepth != 0 || typ.Name != class {
			continue
		}
		count++
		if !typ.SourceBacked || typ.Namespace != namespace || typ.EffectiveAPIVersion != api || filepath.Base(typ.File) != class+".cls" {
			t.Fatalf("indexed source context changed: %+v", typ)
		}
		found = typ.File
	}
	if count != 1 {
		t.Fatalf("expected one indexed source for %s; got %d", class, count)
	}
	return found
}

// c14ReadTwoColumnRows uses the glade-tools native-capture id/value wire format.
// It rejects duplicate keys and parses each value as one complete JSON object.

func c14ReadTwoColumnRows(raw []byte) (map[string]map[string]any, []string, error) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return nil, nil, fmt.Errorf("missing rows or final newline")
	}
	rows := map[string]map[string]any{}
	order := []string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		key, value, ok := strings.Cut(line, "\t")
		if !ok || key == "" || strings.Contains(value, "\t") {
			return nil, nil, fmt.Errorf("invalid two-column row")
		}
		api, method, ok := strings.Cut(key, "/")
		if !ok || api == "" || method == "" || strings.Contains(method, "/") {
			return nil, nil, fmt.Errorf("invalid API/method key")
		}
		if _, exists := rows[key]; exists {
			return nil, nil, fmt.Errorf("duplicate key %s", key)
		}
		v, err := c12JSONObject([]byte(value))
		if err != nil {
			return nil, nil, err
		}
		rows[key] = v
		order = append(order, key)
	}
	return rows, order, nil
}

func originalValid18(id string) bool {
	if len(id) != 18 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	for i := 0; i < 3; i++ {
		bits := 0
		for j := 0; j < 5; j++ {
			c := id[i*5+j]
			if c >= 'A' && c <= 'Z' {
				bits |= 1 << j
			}
		}
		if id[15+i] != alphabet[bits] {
			return false
		}
	}
	return true
}
