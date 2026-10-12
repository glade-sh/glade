package repoguard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The public guard in public_safety_test.go covers personal paths, scratch
// hosts, capture paths under the capture tool's root and snake_case capture
// keys. These rules cover the remaining campaign provenance shapes: private
// org aliases and host names (kept only as salted SHA-256 digests), roadmap
// references in any context, capture job IDs, camelCase revision and capture
// keys, capture revisions in prose, dated capture directories and private
// workspace paths. Findings report a category and a line, never the value.

const campaignNameSalt = "glade repoguard campaign name v1\x00"

type campaignNameKind uint8

const (
	// campaignNamePrefix matches a lower-cased token that starts with the
	// product name and whose first length bytes hash to the digest.
	campaignNamePrefix campaignNameKind = iota
	// campaignNameWord matches a whole ASCII word of length letters.
	campaignNameWord
)

type campaignName struct {
	kind   campaignNameKind
	length int
	digest string
}

// Org aliases, the private roadmap checkout and private host names.
var campaignNameTable = []campaignName{
	{campaignNamePrefix, 13, "3d31c06472b626f4d8440e965b88ff96f6a33d67806d18524ef9d68c90176469"},
	{campaignNamePrefix, 12, "2758141429a1ffa4a95f9846ccc300953af6c1bb8a487849cb81eade94b42f8f"},
	{campaignNamePrefix, 13, "76174ff1f8555d083cc739a7a3c33ed821424607624c466ab1a746d2294139f5"},
	{campaignNamePrefix, 10, "edc0673ef85a2a4cac08289756fb832adb3d70631404d4d4edf1b2e52c2eb3a5"},
	{campaignNamePrefix, 14, "46555a44e6d60149e6160785d822a70b1eae96f8fb3ea5478c432b81c30b233c"},
	{campaignNamePrefix, 10, "67df6346951e88f6e9b3d5768b950b3e274ca11781ad5dca722583f3b3f040f7"},
	{campaignNamePrefix, 13, "548a9fd1ee3cfbbfdd6957b000801724ca7736ba0815a92d90a229b45253f5b0"},
	{campaignNameWord, 5, "274e2f5b915d22324c47a01e1eae9e181daf7efbdf1e20041c2314c868ab41ee"},
	{campaignNameWord, 6, "15a8fa5f800316437846029bc5b2bdebae033e1451e2367762b60503d08cf173"},
	{campaignNameWord, 6, "a213b961372ac555c6f2727909a0a535137d774b576f359c9c8c246492be7ed7"},
	{campaignNameWord, 3, "8f3025879e71cc3837522a87c4ee5117065c5ebb37fd20d3cadc12d9874fac86"},
}

var (
	campaignTicket         = regexp.MustCompile(`\b(?:RO` + `AD|TA` + `SK)-\d+`)
	campaignWorkspacePath  = regexp.MustCompile(`(?:\$HOME|\$\{HOME\}|~)/Dev/([A-Za-z0-9._-]*)`)
	campaignToolPath       = regexp.MustCompile(`(?:^|[\s"'(` + "`" + `=])(?:co` + `ord|sol-` + `runs)/|/ho` + `me/cap` + `ture/`)
	campaignJobID          = regexp.MustCompile(`1a[0-9a-f]{9,10}`)
	campaignProseRevision  = regexp.MustCompile(`(?i)\b(?:tools|probe\.py at|packet at)[\s:=` + "`" + `]+[0-9a-f]{7,40}\b`)
	campaignDatedDirectory = regexp.MustCompile(`\b[a-z][a-z0-9_]*(?:-[a-z0-9_]+)*-20[0-9]{6}[a-z0-9]*/`)
	campaignHexRevision    = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
	campaignCapturedFile   = regexp.MustCompile(`(?i)(?:^|/)[^/]*(?:org[^/]*\.tsv|org[-_]run\.txt)(?:$|[?#])`)
	campaignCamelBoundary  = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

// The sibling maintenance checkout is documented public layout.
const campaignPublicWorkspace = "glade-tools"

func TestPublicCampaignData(t *testing.T) {
	names := campaignNameMatcher(campaignNameTable)
	for _, file := range repoRegularFiles(t, repoRoot(t)) {
		if privateCaptureArtifact(file.rel) {
			continue // TestPublicReleaseData reports these files.
		}
		for _, finding := range publicCampaignFindings(names, file.rel, file.rel) {
			t.Errorf("%s: campaign value in path: %s (REDACTED)", file.rel, finding.kind)
		}
		err := scanPublicAsset(file.rel, file.text, func(member, text string) {
			for _, finding := range publicCampaignFindings(names, file.rel, text) {
				t.Errorf("%s%s:%d: %s (REDACTED)", file.rel, member, finding.line, finding.kind)
			}
		})
		if err != nil {
			t.Errorf("%s: %v", file.rel, err)
		}
	}
}

type campaignNameMatch struct {
	prefixes map[int]map[string]bool
	words    map[int]map[string]bool
	seen     map[*map[int]map[string]bool]map[string]bool
}

func campaignNameMatcher(table []campaignName) *campaignNameMatch {
	m := &campaignNameMatch{prefixes: map[int]map[string]bool{}, words: map[int]map[string]bool{}}
	m.seen = map[*map[int]map[string]bool]map[string]bool{&m.prefixes: {}, &m.words: {}}
	for _, row := range table {
		set := m.prefixes
		if row.kind == campaignNameWord {
			set = m.words
		}
		if set[row.length] == nil {
			set[row.length] = map[string]bool{}
		}
		set[row.length][row.digest] = true
	}
	return m
}

func (m *campaignNameMatch) hit(set *map[int]map[string]bool, name string) bool {
	seen := m.seen[set]
	if hit, ok := seen[name]; ok {
		return hit
	}
	sum := sha256.Sum256([]byte(campaignNameSalt + name))
	hit := (*set)[len(name)][hex.EncodeToString(sum[:])]
	seen[name] = hit
	return hit
}

func campaignWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

func campaignLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// names returns the byte offsets of guarded aliases and host names.
func (m *campaignNameMatch) names(text string) []int {
	var offsets []int
	lower := strings.ToLower(text)
	for i := 0; i < len(lower); {
		if !campaignLetter(lower[i]) || i > 0 && campaignWordByte(lower[i-1]) {
			i++
			continue
		}
		end := i
		for end < len(lower) && campaignLetter(lower[end]) {
			end++
		}
		word := lower[i:end]
		if m.words[len(word)] != nil && (end == len(lower) || !campaignWordByte(lower[end])) && m.hit(&m.words, word) {
			offsets = append(offsets, i)
		}
		if strings.HasPrefix(word, "glade") {
			token := end
			for token < len(lower) && (campaignWordByte(lower[token]) || lower[token] == '-') {
				token++
			}
			for length := range m.prefixes {
				if token-i >= length && m.hit(&m.prefixes, lower[i:i+length]) {
					offsets = append(offsets, i)
					break
				}
			}
			end = token
		}
		i = end
	}
	return offsets
}

func publicCampaignFindings(names *campaignNameMatch, rel, text string) []publicDataFinding {
	text = unicodeEscape.ReplaceAllStringFunc(text, func(s string) string {
		n, _ := strconv.ParseUint(s[2:], 16, 16)
		return string(rune(n))
	})
	text = strings.ReplaceAll(text, `\/`, "/")
	var findings []publicDataFinding
	add := func(offset int, kind string) {
		line := 1 + strings.Count(text[:offset], "\n")
		for _, finding := range findings {
			if finding.line == line && finding.kind == kind {
				return
			}
		}
		findings = append(findings, publicDataFinding{line, kind})
	}
	for _, offset := range names.names(text) {
		add(offset, "private org alias or host name")
	}
	for _, span := range campaignTicket.FindAllStringIndex(text, -1) {
		add(span[0], "campaign roadmap reference")
	}
	for _, span := range campaignWorkspacePath.FindAllStringSubmatchIndex(text, -1) {
		name := text[span[2]:span[3]]
		next := span[3]
		if name == campaignPublicWorkspace && (next == len(text) || text[next] != '/') {
			continue
		}
		add(span[0], "private workspace path")
	}
	for _, span := range campaignToolPath.FindAllStringIndex(text, -1) {
		add(span[0], "campaign tool or capture host path")
	}
	for _, span := range campaignJobID.FindAllStringIndex(text, -1) {
		if span[0] > 0 && campaignWordByte(text[span[0]-1]) || span[1] < len(text) && campaignWordByte(text[span[1]]) && text[span[1]] != '_' {
			continue
		}
		add(span[0], "capture job ID")
	}
	for _, span := range campaignProseRevision.FindAllStringIndex(text, -1) {
		add(span[0], "capture revision in prose")
	}
	for _, span := range campaignDatedDirectory.FindAllStringIndex(text, -1) {
		add(span[0], "dated capture directory")
	}
	if (strings.HasSuffix(rel, ".json") || strings.HasSuffix(rel, ".json.gz")) && json.Valid([]byte(text)) {
		campaignJSONFindings(text, add)
	}
	return findings
}

// campaignKeyWords turns camelCase and snake_case keys into snake_case words.
func campaignKeyWords(key string) string {
	return strings.ToLower(campaignCamelBoundary.ReplaceAllString(strings.ReplaceAll(key, "-", "_"), "${1}_${2}"))
}

func campaignJSONFindings(text string, add func(int, string)) {
	decoder := json.NewDecoder(strings.NewReader(text))
	position := func() int {
		offset := int(decoder.InputOffset())
		return offset + len(text[offset:]) - len(strings.TrimLeft(text[offset:], " \t\r\n:,"))
	}
	var scan func(editorial, revision bool)
	scan = func(editorial, revision bool) {
		offset := position()
		value, err := decoder.Token()
		if err != nil {
			return
		}
		switch value := value.(type) {
		case json.Delim:
			if value == '{' {
				for decoder.More() {
					keyOffset := position()
					token, _ := decoder.Token()
					key, _ := token.(string)
					words := campaignKeyWords(key)
					compact := strings.ReplaceAll(words, "_", "")
					if strings.Contains(compact, "capturejob") || strings.Contains(compact, "orgalias") {
						add(keyOffset, "capture job or org alias field")
					}
					// Release assurance records a public tools SHA, so SHA keys stay with
					// the snake_case rule in public_safety_test.go.
					childRevision := revision || strings.HasSuffix(compact, "commit") || strings.Contains(compact, "toolscheckpoint") ||
						strings.Contains(compact, "toolsrevision")
					childEditorial := editorial || editorialMetadata.MatchString(words)
					if key == "source" || key == "Source" || key == "code" || key == "declarations" {
						childEditorial = false
					}
					scan(childEditorial, childRevision)
				}
				_, _ = decoder.Token()
			} else if value == '[' {
				for decoder.More() {
					scan(editorial, revision)
				}
				_, _ = decoder.Token()
			}
		case string:
			if revision && campaignHexRevision.MatchString(value) {
				add(offset, "capture revision field")
			}
			if editorial && strings.Contains(value, "/") && campaignCapturedFile.MatchString(value) {
				add(offset, "capture file path in provenance field")
			}
		}
	}
	scan(false, false)
}

func TestPublicCampaignMatchers(t *testing.T) {
	// Fixture names are hashed here exactly as the real table is.
	sum := func(name string) string {
		digest := sha256.Sum256([]byte(campaignNameSalt + name))
		return hex.EncodeToString(digest[:])
	}
	names := campaignNameMatcher([]campaignName{
		{campaignNamePrefix, len("glade-example-"), sum("glade-example-")},
		{campaignNameWord, len("hostx"), sum("hostx")},
	})
	ticket := "RO" + "AD-12"
	tests := []struct {
		name, rel, text string
		count           int
	}{
		{"alias", "a.go", "org glade-example-20990101a", 1},
		{"alias upper", "a.md", "GLADE-EXAMPLE-01", 1},
		{"alias needs suffix", "a.md", "glade-example", 0},
		{"product name", "a.md", "glade-tools and glade-sh", 0},
		{"alias inside word", "a.md", "xglade-example-01", 0},
		{"host", "a.md", "runs on Hostx today", 1},
		{"host in path", "a.md", "/srv/hostx/run", 1},
		{"longer word", "a.md", "hostxy", 0},
		{"host with digit", "a.md", "hostx1", 0},
		{"ticket in string", "a.go", `owner := "` + ticket + `"`, 1},
		{"ticket in json", "testdata/a.json", `{"input":"` + ticket + `"}`, 1},
		{"task ticket", "a.md", "TA" + "SK-9.65", 1},
		{"workspace", "a.md", "$HOME/" + "Dev/private-board", 1},
		{"tilde workspace", "a.md", "~/" + "Dev/x", 1},
		{"sibling tools", "AGENTS.md", "`~/" + "Dev/glade-tools` project", 0},
		{"sibling tools subpath", "AGENTS.md", "~/" + "Dev/glade-tools/private", 1},
		{"coordinator path", "a.json", `"co` + `ord/api-hub/native.json"`, 1},
		{"capture home", "a.json", `"captures/ho` + `me/cap` + `ture/run"`, 1},
		{"coordinate word", "a.go", "x.co" + "ord/2", 0},
		{"job", "a.json", `"1a` + `12c65f3e3-API68-capture"`, 1},
		{"job in prose", "a.go", "// capture 1a" + "12bb0b882.", 1},
		{"long hex", "a.json", `"1a` + `12c65f3e3abcdef0"`, 0},
		{"prose tools revision", "a.go", "// from tools " + "e2d118dd and", 1},
		{"prose probe revision", "a.json", `"probe.py at ` + `a567d499"`, 1},
		{"tools word", "a.md", "tools 2026", 0},
		{"dated directory", "a.json", `"annotations-` + `20991005/integer-api62"`, 1},
		{"date without directory", "a.md", "released 20991005", 0},
		{"camel tools commit", "testdata/a.json", `{"controlsToolsCommit":"` + strings.Repeat("b", 40) + `"}`, 1},
		{"camel oracle commit", "testdata/a.json", `{"searchOracleCommit":"` + strings.Repeat("b", 8) + `"}`, 1},
		{"checkpoint", "testdata/a.json", `{"toolsCheckpoint":"` + strings.Repeat("c", 9) + `"}`, 1},
		{"descriptive commit", "testdata/a.json", `{"toolsCommit":"native capture"}`, 0},
		{"nested revision", "testdata/a.json", `{"ceiling":{"ToolsCommit":"` + strings.Repeat("d", 40) + `"}}`, 1},
		{"camel job", "testdata/a.json", `{"CaptureJob":"x"}`, 1},
		{"camel alias key", "testdata/a.json", `{"OrgAlias":"x"}`, 1},
		{"camel provenance path", "testdata/a.json", `{"primaryOracle":"capture/api68/org.tsv"}`, 1},
		{"camel source path", "testdata/a.json", `{"controlsSource":"api68 org.tsv + run/ORG-RUN.txt"}`, 1},
		{"row file", "testdata/a.json", `{"rows":"api68/org.tsv"}`, 0},
		{"source digest", "testdata/a.json", `{"sha256":"` + strings.Repeat("e", 64) + `"}`, 0},
		{"release tools SHA", "site/release-manifest.json", `{"toolsSHA":"` + strings.Repeat("f", 40) + `"}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := publicCampaignFindings(names, tt.rel, tt.text); len(got) != tt.count {
				t.Fatalf("finding count = %d, want %d (%v)", len(got), tt.count, got)
			}
		})
	}
}

func TestPublicCampaignNameTable(t *testing.T) {
	seen := map[string]bool{}
	for i, row := range campaignNameTable {
		raw, err := hex.DecodeString(row.digest)
		if err != nil || len(raw) != sha256.Size || row.length <= 0 || seen[row.digest] {
			t.Errorf("campaign name %d is malformed or repeated", i+1)
		}
		seen[row.digest] = true
	}
}

func TestPublicCampaignGuardSourceIsPortable(t *testing.T) {
	const rel = "internal/repoguard/public_campaign_test.go"
	data, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatal(err)
	}
	if got := publicDataFindings(rel, string(data)); len(got) != 0 {
		t.Fatalf("guard source must not embed private-looking values: %v", got)
	}
	if got := publicCampaignFindings(campaignNameMatcher(campaignNameTable), rel, string(data)); len(got) != 0 {
		t.Fatalf("guard source must not embed campaign values: %v", got)
	}
}
