package apextest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// accountCaptureNativeRows reads an org.tsv with the header api, route, id,
// observation into api -> route -> ordered rows.
func accountCaptureNativeRows(t *testing.T, fixture string) map[string]map[string][][2]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if lines[0] != "api\troute\tid\tobservation" {
		t.Fatalf("org.tsv header %q", lines[0])
	}
	out := map[string]map[string][][2]string{}
	for _, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 4 {
			t.Fatalf("malformed row %q", line)
		}
		if out[cells[0]] == nil {
			out[cells[0]] = map[string][][2]string{}
		}
		out[cells[0]][cells[1]] = append(out[cells[0]][cells[1]], [2]string{cells[2], cells[3]})
	}
	return out
}

// accountCaptureProject copies a fixture project, setting every 67.0 to the API.
func accountCaptureProject(t *testing.T, fixture, api string) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(fixture, "project")
	version := regexp.MustCompile(`67\.0`)
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(source, path)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, rel), version.ReplaceAllString(string(raw), api))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// accountCaptureRows parses ROW|id|observation debug lines, and the ROWS>...<ROWS
// message of a probe test.
func accountCaptureDebugRows(lines []string) map[string]string {
	got := map[string]string{}
	for _, line := range lines {
		if _, row, ok := strings.Cut(line, "ROW|"); ok {
			id, observation, _ := strings.Cut(row, "|")
			got[id] = normalizeAccountCapture(observation)
		}
	}
	return got
}

var (
	accountCaptureRecordTypeID = regexp.MustCompile(`\b012[A-Za-z0-9]{12}(?:[A-Za-z0-9]{3})?\b`)
	accountCaptureCaseNumber   = regexp.MustCompile(`cases\.: \d+$`)
)

// normalizeAccountCapture applies the capture extraction masks (native-run.md).
func normalizeAccountCapture(observation string) string {
	observation = strings.TrimRight(observation, " \n")
	observation = accountCaptureRecordTypeID.ReplaceAllString(observation, "<recordTypeId>")
	return accountCaptureCaseNumber.ReplaceAllString(observation, "cases.: <caseNumber>")
}

func accountCaptureTestRows(t *testing.T, run testreport.Run) map[string]string {
	t.Helper()
	var message string
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				message = c.Problem.Message
			}
		}
	}
	_, body, ok := strings.Cut(message, "ROWS>")
	body, _, closed := strings.Cut(body, "<ROWS")
	if !ok || !closed {
		t.Fatalf("isTest: no row message: %q", message)
	}
	got := map[string]string{}
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		id, observation, _ := strings.Cut(line, "|")
		got[id] = normalizeAccountCapture(observation)
	}
	return got
}
