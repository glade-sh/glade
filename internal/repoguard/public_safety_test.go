package repoguard

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	personalHomePath   = regexp.MustCompile("(?m)(?:^|[\\s\"'`=(<:])(?:file://)?(?:/(?:Users|home|Volumes)/[^/\\s\"'`<>]+(?:/[^\\s\"'`<>]*)?|[A-Za-z]:[/\\\\]+Users[/\\\\]+[^/\\\\\\s\"'`<>]+)")
	personalMailbox    = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@(?:gmail|googlemail|yahoo|hotmail|outlook|live|icloud|me|aol|protonmail|proton)\.(?:com|net|me)\b`)
	recordIdentifier   = regexp.MustCompile(`[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?`)
	unicodeEscape      = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)
	scratchHostname    = regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9-]*\.scratch\.(?:my\.salesforce|lightning\.force|my\.site|vf\.force)\.com\b`)
	compilerFilesystem = regexp.MustCompile(`/ho` + `me/[^/\s"'<>]+/tools/[^\s"'<>]+`)
	campaignReference  = regexp.MustCompile(`\b(?:RO` + `AD-\d+|TA` + `SK-\d+(?:\.\d+)?|[ALVB]\d{2}|D(?:[1-9]|1[0-9]))\b`)
	privateCommit      = regexp.MustCompile(`\b[0-9a-fA-F]{7,40}\b`)
	privateCapturePath = regexp.MustCompile(`(?:/|\b)family` + `probe/[^\s"'<>]+`)
	editorialMetadata  = regexp.MustCompile(`(?i)(?:^|_)(?:notes?|reason|review|oracle|basis|family|surface|owner|description|comment|provenance|decision|boundary|status|explanation|source|sources|evidence)(?:$|_)`)
)

// Salesforce's own LWC compiler location appears in native diagnostics, and
// glade reproduces it for parity. Only this exact prefix is allowed.
const salesforceLWCCompilerPrefix = "/ho" + "me/sfdc/tools/sfdc-lwc-compiler/"

type publicDataFinding struct {
	line int
	kind string
}

// Check decoded assets as well as text: embedded catalogs are part of the
// distributed product even though a plain text search cannot inspect them.
func TestPublicReleaseData(t *testing.T) {
	for _, file := range repoRegularFiles(t, repoRoot(t)) {
		if privateCaptureArtifact(file.rel) {
			t.Errorf("%s:1: private capture or preparation material (REDACTED)", file.rel)
		}
		err := scanPublicAsset(file.rel, file.text, func(member, text string) {
			for _, finding := range publicDataFindings(file.rel, text) {
				// Report categories and locations, never the captured value itself.
				t.Errorf("%s%s:%d: %s (REDACTED)", file.rel, member, finding.line, finding.kind)
			}
		})
		if err != nil {
			t.Errorf("%s: %v", file.rel, err)
		}
	}
}

func privateCaptureArtifact(rel string) bool {
	base := filepath.Base(rel)
	return base == "AGENTS.override.md" ||
		(strings.HasPrefix(base, "SERIALIZATION-") && strings.HasSuffix(base, "-RECEIPT.md")) ||
		strings.HasSuffix(base, "ORG-RUN.txt") ||
		base == "capture-context.json" || base == "capture-state.json" ||
		base == "capture-status.json" || base == "capture-unobserved.json" ||
		rel == "testdata/conformance/string/string_cases.py"
}

func publicFixturePath(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if part == "testdata" || part == "test" || part == "tests" || part == "__tests__" {
			return true
		}
	}
	return strings.HasSuffix(rel, "_test.go") || strings.Contains(filepath.Base(rel), ".test.")
}

func publicDataFindings(rel, text string) []publicDataFinding {
	// JSON can encode identifying text with Unicode or escaped slashes.
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
	for _, span := range personalHomePath.FindAllStringIndex(text, -1) {
		match := text[span[0]:span[1]]
		trimmed := strings.TrimLeft(match, " \t\r\n\"'`=(<:")
		path := strings.TrimPrefix(trimmed, "file://")
		// A JSON-escaped quote leaves its backslash at this match boundary.
		path = strings.TrimRight(path, `\`)
		// Hosted cache paths and product routes are portable. Compiler paths
		// have a separate category so their diagnostic meaning can be retained.
		if strings.HasPrefix(path, "/home/runner/") || strings.HasPrefix(path, "/Users/runner/") ||
			compilerFilesystem.MatchString(path) || path == "/home/home.jsp" || path == "/home/contours.svg" {
			continue
		}
		add(span[0]+len(match)-len(trimmed), "personal absolute path")
	}
	for _, rule := range []struct {
		pattern *regexp.Regexp
		kind    string
	}{
		{personalMailbox, "personal-provider mailbox"},
		{scratchHostname, "scratch org hostname"},
		{compilerFilesystem, "compiler implementation filesystem path"},
		{privateCapturePath, "private capture provenance path"},
	} {
		for _, span := range rule.pattern.FindAllStringIndex(text, -1) {
			if rule.pattern == scratchHostname && publicSyntheticScratchHost(rel, text, span) {
				continue
			}
			if rule.pattern == compilerFilesystem && strings.HasPrefix(text[span[0]:span[1]], salesforceLWCCompilerPrefix) {
				continue
			}
			add(span[0], rule.kind)
		}
	}
	// Tests own deliberately noncanonical ID inputs. Runtime code, docs and
	// embedded catalogs use the synthetic namespace. Captured UI fixtures must
	// also be checked: their JSON includes native metadata and user identities.
	if !publicFixturePath(rel) || publicCapturedFixturePath(rel) {
		for _, span := range recordIdentifier.FindAllStringIndex(text, -1) {
			id := text[span[0]:span[1]]
			// Underscores delimit IDs embedded in exported metadata names.
			// Letters, digits and '+' instead indicate a larger token or base64.
			if (span[0] > 0 && publicIDTokenByte(text[span[0]-1])) || (span[1] < len(text) && publicIDTokenByte(text[span[1]])) || !publicRecordIDCandidate(id) {
				continue
			}
			// Synthetic defaults use nine zeroes followed by a three-character
			// sequence (including the existing guest/event-user suffixes).
			if strings.HasPrefix(id[3:15], "000000000") || publicOwnedAdapterID(rel, id) {
				continue
			}
			add(span[0], fmt.Sprintf("nonsynthetic %d-char org or record ID", len(id)))
		}
	}
	publicEditorialFindings(rel, text, add)
	return findings
}

func publicOwnedAdapterID(rel, id string) bool {
	// This adapter fixture assigns seven sequential numeric record inputs;
	// observed records are normalized independently to owned-result labels.
	return rel == "internal/lwc/compile/testdata/l07_review_adapter.json" &&
		(len(id) == 15 || (len(id) == 18 && id[15:] == "AAA")) &&
		strings.HasPrefix(id, "001"+"00000007010") && id[14] >= '1' && id[14] <= '7'
}

func publicSyntheticScratchHost(rel, text string, span []int) bool {
	host := text[span[0]:span[1]]
	switch rel {
	case "internal/vm/stdlib_test.go":
		return host == "owned"+".scratch.my.salesforce.com" || host == "owned--c"+".scratch.vf.force.com"
	case "internal/apextest/url_domain_conformance_test.go":
		return host == "url-domain"+".scratch.my.salesforce.com"
	case "testdata/conformance/url_domain/cases.json":
		return strings.HasSuffix(text[:span[0]], "${ORG_LABEL}--") &&
			(host == "c"+".scratch.vf.force.com" || host == "pkg"+".scratch.vf.force.com")
	}
	return false
}

func publicCapturedFixturePath(rel string) bool {
	if !strings.HasSuffix(rel, ".json") && !strings.HasSuffix(rel, ".json.gz") && !strings.HasSuffix(rel, ".resource") && !strings.HasSuffix(rel, ".zip") {
		return false
	}
	for _, prefix := range []string{"internal/lwc/", "internal/lwcbrowser/", "internal/server/", "internal/visualforce/"} {
		if strings.HasPrefix(rel, prefix) && strings.Contains(rel, "/testdata/") {
			return true
		}
	}
	return false
}

func publicEditorialFindings(rel, text string, add func(int, string)) {
	// Parse comments so fixture identifiers and diagnostic strings remain data.
	commentPattern := campaignReference
	if publicFixturePath(rel) {
		commentPattern = regexp.MustCompile(`\b(?:RO` + `AD-\d+|TA` + `SK-\d+(?:\.\d+)?)\b`)
	}
	if strings.HasSuffix(rel, ".go") && commentPattern.MatchString(text) {
		fset := token.NewFileSet()
		file, _ := parser.ParseFile(fset, rel, text, parser.ParseComments)
		if file != nil {
			for _, group := range file.Comments {
				for _, comment := range group.List {
					for _, span := range commentPattern.FindAllStringIndex(comment.Text, -1) {
						add(fset.Position(comment.Pos()).Offset+span[0], "campaign reference in source comment")
					}
				}
			}
		}
	}
	if strings.HasSuffix(rel, ".mjs") || strings.HasSuffix(rel, ".js") || strings.HasSuffix(rel, ".ts") || (strings.HasSuffix(rel, ".go") && !publicFixturePath(rel)) {
		offset := 0
		for _, line := range strings.SplitAfter(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
				for _, span := range commentPattern.FindAllStringIndex(line, -1) {
					add(offset+span[0], "campaign reference in source comment")
				}
			}
			offset += len(line)
		}
	}
	// Describe catalogs have no editorial fixture fields; IDs, paths, mailboxes
	// and hostnames above still inspect every decoded catalog member.
	if !publicFixturePath(rel) || (!strings.HasSuffix(rel, ".json") && !strings.HasSuffix(rel, ".json.gz")) || !json.Valid([]byte(text)) {
		return
	}
	// Captured executable source can contain family-shaped literal inputs.
	// Its companion digest identifies the exact owned source; provenance
	// strings without that matching digest remain editorial metadata.
	decoder := json.NewDecoder(strings.NewReader(text))
	// These capture labels are ornamental. Other XML labels are behavioral
	// schema/list-view inputs and may intentionally contain fixture case names.
	label := regexp.MustCompile(`<label>\s*V[0-9]{2}\s+(?:owned (?:row|oracle)|oracle access [0-9]+)\s*</label>`)
	capturedFile := regexp.MustCompile(`(?i)(?:^|/)[^/]*(?:org[^/]*\.tsv|org[-_]run\.txt)(?:$|[?#])`)
	position := func() int {
		offset := int(decoder.InputOffset())
		return offset + len(text[offset:]) - len(strings.TrimLeft(text[offset:], " \t\r\n:,"))
	}
	ownedSources := make(map[int]bool)
	var collectOwnedSources func() (string, int)
	collectOwnedSources = func() (string, int) {
		offset := position()
		value, err := decoder.Token()
		if err != nil {
			return "", offset
		}
		switch value := value.(type) {
		case string:
			return value, offset
		case json.Delim:
			if value == '{' {
				var source, digest string
				var sourceOffset int
				for decoder.More() {
					key, _ := decoder.Token()
					child, childOffset := collectOwnedSources()
					if key == "source" {
						source, sourceOffset = child, childOffset
					} else if key == "sourceSHA256" {
						digest = child
					}
				}
				_, _ = decoder.Token()
				if source != "" && digest == fmt.Sprintf("%x", sha256.Sum256([]byte(source))) {
					ownedSources[sourceOffset] = true
				}
			} else if value == '[' {
				for decoder.More() {
					collectOwnedSources()
				}
				_, _ = decoder.Token()
			}
		}
		return "", offset
	}
	collectOwnedSources()
	decoder = json.NewDecoder(strings.NewReader(text))
	var scan func(bool, bool, bool)
	scan = func(editorial, revision, source bool) {
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
					keyToken, _ := decoder.Token()
					key := keyToken.(string)
					switch key {
					case "old_tickets", "capture_job", "orgTsv", "orgRun", "sourceSnapshot", "org_tsv", "org_run":
						add(keyOffset, "private capture or campaign metadata field")
					}
					lower := strings.ToLower(key)
					privateRevision := strings.Contains(lower, "oracle_commit") || strings.Contains(lower, "tools_commit") || strings.Contains(lower, "tools_capture_commit") || lower == "tools_sha" || lower == "sourcecommit"
					childEditorial := editorial || editorialMetadata.MatchString(key)
					if key == "Source" || key == "code" || key == "declarations" {
						childEditorial = false
					}
					scan(childEditorial, revision || privateRevision, key == "source")
				}
				_, _ = decoder.Token()
			} else if value == '[' {
				for decoder.More() {
					scan(editorial, revision, source)
				}
				_, _ = decoder.Token()
			}
		case string:
			if revision && privateCommit.MatchString(value) {
				add(offset, "private capture revision")
			}
			if editorial && !(source && ownedSources[offset]) && campaignReference.MatchString(value) {
				add(offset, "campaign reference in fixture metadata")
			}
			if editorial && capturedFile.MatchString(value) {
				add(offset, "private capture provenance path")
			}
			for _, match := range label.FindAllString(value, -1) {
				if campaignReference.MatchString(match) {
					add(offset, "campaign reference in embedded fixture label")
				}
			}
		}
	}
	scan(false, false, false)
}

func publicIDTokenByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+'
}

func publicRecordIDCandidate(id string) bool {
	if len(id) == 15 {
		// Short IDs have no checksum. Restrict those ambiguous shapes to
		// common record, org, user, metadata and custom-object key prefixes.
		switch id[:3] {
		case "00D", "00B", "005", "012", "001", "003", "006", "00Q", "00T", "00U", "500", "701", "707", "01p", "01q", "01I", "00h", "01D", "01B", "01N", "0PS", "02d", "7zi":
			return true
		}
		return strings.HasPrefix(id, "a0")
	}
	if len(id) != 18 || !((id[0] >= '0' && id[0] <= '9') || strings.ContainsRune("aAkK", rune(id[0]))) {
		return false
	}
	// Long IDs carry three case-mask characters. Validate them so numeric
	// constants, hashes, SVG coordinates and encoded assets are not IDs.
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	for block := 0; block < 3; block++ {
		mask := 0
		for bit := 0; bit < 5; bit++ {
			c := id[block*5+bit]
			if c >= 'A' && c <= 'Z' {
				mask |= 1 << bit
			}
		}
		if id[15+block] != alphabet[mask] {
			return false
		}
	}
	return true
}

// Packs contain a 16-byte header followed by independent gzip members. Reading
// each member (including all members of ordinary gzip assets) prevents binary
// NULs, an unexamined second member, or a corrupt payload from hiding findings.
func scanPublicAsset(rel, data string, visit func(string, string)) error {
	const maxDecodedBytes = 512 << 20
	if strings.HasSuffix(rel, ".zip") || strings.HasPrefix(data, "PK\x03\x04") || strings.HasPrefix(data, "PK\x05\x06") {
		archive, err := zip.NewReader(strings.NewReader(data), int64(len(data)))
		if err != nil {
			return fmt.Errorf("invalid ZIP asset")
		}
		remaining := int64(maxDecodedBytes)
		for i, file := range archive.File {
			if file.FileInfo().IsDir() {
				continue
			}
			reader, err := file.Open()
			if err != nil {
				return fmt.Errorf("cannot open ZIP member %d", i+1)
			}
			raw, readErr := io.ReadAll(io.LimitReader(reader, remaining+1))
			closeErr := reader.Close()
			if readErr != nil || closeErr != nil {
				return fmt.Errorf("cannot decode ZIP member %d", i+1)
			}
			remaining -= int64(len(raw))
			if remaining < 0 {
				return fmt.Errorf("decoded asset exceeds %d bytes", maxDecodedBytes)
			}
			// Archive paths can also contain identities. Use ordinal locations
			// in findings so a private filename never reaches the test output.
			visit(fmt.Sprintf(" (ZIP member %d name)", i+1), file.Name)
			if strings.IndexByte(string(raw), 0) < 0 {
				visit(fmt.Sprintf(" (ZIP member %d)", i+1), string(raw))
			}
		}
		return nil
	}
	reader := strings.NewReader(data)
	wantMembers := -1
	if strings.HasSuffix(rel, ".pack") {
		if len(data) < 16 || (data[:8] != "GLADEC2\x00" && data[:8] != "GLADER2\x00") ||
			binary.BigEndian.Uint32([]byte(data[8:12])) != 2 {
			return fmt.Errorf("unsupported or corrupt compressed pack")
		}
		wantMembers = int(binary.BigEndian.Uint32([]byte(data[12:16])))
		reader.Seek(16, io.SeekStart)
	} else if !strings.HasSuffix(rel, ".gz") && !(len(data) >= 2 && data[:2] == "\x1f\x8b") {
		if strings.IndexByte(data, 0) < 0 {
			visit("", data)
		}
		return nil
	}
	remaining := int64(maxDecodedBytes)
	members := 0
	for reader.Len() > 0 {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return fmt.Errorf("invalid gzip member %d: %w", members+1, err)
		}
		gz.Multistream(false)
		raw, err := io.ReadAll(io.LimitReader(gz, remaining+1))
		closeErr := gz.Close()
		if err != nil {
			return fmt.Errorf("decode gzip member %d: %w", members+1, err)
		}
		if closeErr != nil {
			return closeErr
		}
		remaining -= int64(len(raw))
		if remaining < 0 {
			return fmt.Errorf("decoded asset exceeds %d bytes", maxDecodedBytes)
		}
		members++
		visit(fmt.Sprintf(" (member %d)", members), string(raw))
	}
	if members == 0 || (wantMembers >= 0 && members != wantMembers) {
		return fmt.Errorf("compressed member count %d does not match header %d", members, wantMembers)
	}
	return nil
}

func TestPublicDataMatchers(t *testing.T) {
	ownedSource := "public class " + "A" + "99 { String input = 'owned'; }"
	ownedDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(ownedSource)))
	ownedSourceWithMailbox := ownedSource + " // person" + "@gmail.com"
	tests := []struct {
		name, rel, text string
		count           int
	}{
		{"mac home", "docs/a.md", "/Users/" + "person/project", 1},
		{"linux home", "a.go", "/home/" + "person/project", 1},
		{"volume", "a.go", "/Volumes/" + "Personal/project", 1},
		{"windows home", "a.go", `C:\Users\` + `Person\project`, 1},
		{"windows slash home", "a.go", "C:/Users/" + "Person/project", 1},
		{"file URI", "a.go", "file:///Users/" + "person/project", 1},
		{"markup path", "docs/a.md", "</Users/" + "person/project>", 1},
		{"colon path", "a.go", "path:/Users/" + "person/project", 1},
		{"hosted cache", "ci.yml", "/home/runner/.cache/tool", 0},
		{"route", "a.go", "/lwc/preview/home/Custom_Home", 0},
		{"home route", "a.go", "/home/home.jsp", 0},
		{"escaped home route", "testdata/a.json", `{"dom":"<a href=\"/home/home.jsp\">Home</a>"}`, 0},
		{"site home asset", "a.vue", "url('/home/contours.svg')", 0},
		{"mailbox in fixture", "testdata/a.json", "person" + "@gmail.com", 1},
		{"escaped mailbox", "a.json", `"person\u0040` + `gmail.com"`, 1},
		{"reserved email", "a.go", "person@example.com", 0},
		{"public contact", "a.go", "hello@glade.sh", 0},
		{"org", "a.go", "00D" + "AbCdEfGhIjKl", 1},
		{"record type", "a.go", "012" + "AbCdEfGhIjKlIVK", 1},
		{"bad ID checksum", "a.go", "012" + "AbCdEfGhIjKlAAA", 0},
		{"custom record", "a.go", "a01" + "AbCdEfGhIjKl", 1},
		{"class record", "a.go", "01p" + "AbCdEfGhIjKl", 1},
		{"permission set", "a.go", "0PS" + "AbCdEfGhIjKl", 1},
		{"metadata picklist", "a.json", "7zi" + "AbCdEfGhIjKl", 1},
		{"short zero prefix", "a.go", "00100000" + "AbCdEfG", 1},
		{"fixture record", "testdata/a.json", "001" + "AbCdEfGhIjKl", 0},
		{"test record", "a_test.go", "001" + "AbCdEfGhIjKl", 0},
		{"captured short record", "internal/lwc/compile/testdata/native.json", `{"id":"001` + `AbCdEfGhIjKl"}`, 1},
		{"captured metadata", "internal/lwc/compile/testdata/native.json", `{"id":"01D` + `AbCdEfGhIjKl"}`, 1},
		{"captured list view", "internal/visualforce/testdata/native.json", `{"id":"00B` + `AbCdEfGhIjKl"}`, 1},
		{"captured long record", "internal/visualforce/testdata/native.json", `{"id":"012` + `AbCdEfGhIjKlIVK"}`, 1},
		{"captured synthetic record", "internal/server/testdata/native.json", `{"id":"001000000000001AAA"}`, 0},
		{"owned ID conversion input", "internal/apextest/testdata/conformance/id/cases.json", `{"input":"001` + `AbCdEfGhIjKl"}`, 0},
		{"owned adapter input", "internal/lwc/compile/testdata/l07_review_adapter.json", "001" + "000000070101AAA", 0},
		{"adapter shape elsewhere", "internal/lwc/compile/testdata/native.json", "001" + "000000070101AAA", 1},
		{"unexpected adapter identity", "internal/lwc/compile/testdata/l07_review_adapter.json", "001" + "000000070108AAA", 1},
		{"synthetic", "a.go", "001000000000001AAA", 0},
		{"master", "a.go", "012000000000000AAA", 0},
		{"larger token", "a.go", "prefix001AbCdEfGhIjKlsuffix", 0},
		{"metadata name ID", "a.json", "Record_type_for_community_001" + "AbCdEfGhIjKlIVK_entity_Idea", 1},
		{"hex constant", "a.go", "0x123456789abcd", 0},
		{"timestamp", "a.go", "201204050607890", 0},
		{"base64 fragment", "a.go", "+001" + "AbCdEfGhIjKl+", 0},
		{"scratch hostname", "testdata/a.json", "https://example" + ".scratch.my.salesforce.com", 1},
		{"scratch lightning hostname", "testdata/a.json", "https://example" + ".scratch.lightning.force.com", 1},
		{"owned domain parser fixture", "internal/vm/stdlib_test.go", "owned" + ".scratch.my.salesforce.com", 0},
		{"owned domain elsewhere", "testdata/a.json", "owned" + ".scratch.my.salesforce.com", 1},
		{"domain placeholder", "testdata/conformance/url_domain/cases.json", "${ORG_LABEL}--c" + ".scratch.vf.force.com", 0},
		{"captured domain near placeholder", "testdata/conformance/url_domain/cases.json", "${ORG_LABEL} https://example" + ".scratch.my.salesforce.com", 1},
		{"public Salesforce docs", "testdata/a.json", "https://developer.salesforce.com/docs", 0},
		{"compiler implementation", "testdata/a.json", "/home/" + "compiler/tools/build/compiler.js", 1},
		{"salesforce lwc compiler", "testdata/a.json", "/home/" + "sfdc/tools/sfdc-lwc-compiler/14.192.8388608/c.js", 0},
		{"other salesforce tool", "testdata/a.json", "/home/" + "sfdc/tools/other-compiler/c.js", 1},
		{"lookalike compiler prefix", "a.mjs", "/home/" + "sfdc/tools/sfdc-lwc-compiler-next/c.js", 1},
		{"compiler diagnostic placeholder", "testdata/a.json", "LWC1503: <compiler>/compiler.js: unexpected token", 0},
		{"product comment", "a.go", "package a\n// covered by " + "RO" + "AD-999\n", 1},
		{"product family comment", "a.mjs", "// implements family " + "L" + "99\n", 1},
		{"product decision comment", "a.go", "package a\n// decision " + "D" + "9\n", 1},
		{"diagnostic case comment", "a.go", "package a\n// case D103\n", 0},
		{"embedded script comment", "a.go", "package a\nconst script = `\n// family " + "L" + "99\n`\n", 1},
		{"test roadmap comment", "a_test.go", "package a\n// see " + "RO" + "AD-999\n", 1},
		{"fixture identifier", "a.go", "package a\nconst fixture = \"FamilyL99Controller\"\n", 0},
		{"fixture case identifier", "testdata/a.json", `{"id":"L99-boundary","input":"FamilyL99Controller"}`, 0},
		{"fixture family metadata", "testdata/a.json", `{"family":"` + "L" + `99"}`, 1},
		{"fixture campaign note", "testdata/a.json", `{"review_note":"tracked in ` + "RO" + `AD-999"}`, 1},
		{"nested fixture metadata", "testdata/a.json", `{"notes":{"59.0":"tracked in ` + "RO" + `AD-999"}}`, 1},
		{"executable source", "testdata/a.json", `{"Source":"class ` + "A" + `99 {}"}`, 0},
		{"hashed executable source", "testdata/a.json", fmt.Sprintf(`{"source":%q,"sourceSHA256":%q}`, ownedSource, ownedDigest), 0},
		{"hashed source identities remain checked", "testdata/a.json", fmt.Sprintf(`{"source":%q,"sourceSHA256":"%x"}`, ownedSourceWithMailbox, sha256.Sum256([]byte(ownedSourceWithMailbox))), 1},
		{"unverified source metadata", "testdata/a.json", fmt.Sprintf(`{"source":%q,"sourceSHA256":"invalid"}`, ownedSource), 1},
		{"duplicate source with invalid local digest", "testdata/a.json", fmt.Sprintf(`{"captured":{"source":%q,"sourceSHA256":%q},"notes":{"source":%q,"sourceSHA256":"invalid"}}`, ownedSource, ownedDigest, ownedSource), 1},
		{"duplicate source without local digest", "testdata/a.json", fmt.Sprintf(`{"captured":{"source":%q,"sourceSHA256":%q},"notes":{"source":%q}}`, ownedSource, ownedDigest, ownedSource), 1},
		{"hashed source elsewhere is still metadata", "testdata/a.json", fmt.Sprintf(`{"notes":%q,"captured":{"source":%q,"sourceSHA256":%q}}`, ownedSource, ownedSource, ownedDigest), 1},
		{"embedded metadata label", "testdata/a.json", `{"files":{"page-meta.xml":"<label>` + "V" + `99 owned row</label>"}}`, 1},
		{"embedded oracle label", "testdata/a.json", `{"files":{"page-meta.xml":"<label>` + "V" + `99 owned oracle</label>"}}`, 1},
		{"embedded oracle access label", "testdata/a.json", `{"files":{"permissionset.xml":"<label>` + "V" + `99 oracle access 59</label>"}}`, 1},
		{"semantic XML fixture label", "testdata/a.json", `{"input":"<label>` + "A" + `99 test field</label>"}`, 0},
		{"semantic list view label", "testdata/a.json", `{"files":{"view.xml":"<label>` + "L" + `11 list view</label>"}}`, 0},
		{"Salesforce Campaign", "testdata/a.json", `{"description":"Campaign and CampaignMember records"}`, 0},
		{"capture revision", "testdata/a.json", `{"tools_commit":"` + strings.Repeat("a", 40) + `"}`, 1},
		{"short capture revision", "testdata/a.json", `{"oracle_commit":"` + strings.Repeat("a", 8) + `"}`, 1},
		{"descriptive oracle", "testdata/a.json", `{"oracle_commit":"Salesforce API capture"}`, 0},
		{"capture job metadata", "testdata/a.json", `{"capture_` + `job":"example"}`, 1},
		{"capture reference", "testdata/a.json", `{"org` + `Tsv":"example.tsv"}`, 1},
		{"capture reference path", "a.go", `"family` + `probe/runs/example/org.tsv"`, 1},
		{"dated capture reference", "a.go", `"family` + `probe/20990101-example/org.tsv"`, 1},
		{"nested capture source", "testdata/a.json", `{"sources":{"59.0":{"compile":"capture/org.tsv"}}}`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := publicDataFindings(tt.rel, tt.text); len(got) != tt.count {
				t.Fatalf("finding count = %d, want %d (%v)", len(got), tt.count, got)
			}
		})
	}
}

func TestPublicDataFindingLocations(t *testing.T) {
	mailbox := "person" + "@gmail.com"
	findings := publicDataFindings("testdata/a.json", "{\n  \"email\": \""+mailbox+"\"\n}\n")
	if len(findings) != 1 || findings[0].line != 2 || strings.Contains(fmt.Sprint(findings), mailbox) {
		t.Fatalf("expected one redacted finding on line 2; got %v", findings)
	}
}

func TestPublicGuardSourceIsPortable(t *testing.T) {
	const rel = "internal/repoguard/public_safety_test.go"
	data, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatal(err)
	}
	if got := publicDataFindings(rel, string(data)); len(got) != 0 {
		t.Fatalf("guard controls must not embed private-looking values: %v", got)
	}
}

func TestPublicAssetScanning(t *testing.T) {
	gzipText := func(text string) string {
		var out bytes.Buffer
		w := gzip.NewWriter(&out)
		if _, err := w.Write([]byte(text)); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	zipText := func(contents ...string) string {
		var out bytes.Buffer
		archive := zip.NewWriter(&out)
		for i, content := range contents {
			file, err := archive.Create(fmt.Sprintf("asset-%d.txt", i))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(file, content); err != nil {
				t.Fatal(err)
			}
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	safe := gzipText(`{"email":"safe@example.com"}`)
	private := gzipText(`{"email":"person` + `@gmail.com"}`)
	id := gzipText(`{"recordTypeId":"012` + `AbCdEfGhIjKlIVK"}`)
	archive := zipText("safe@example.com", "person"+"@gmail.com")
	header := make([]byte, 16)
	copy(header, "GLADEC2\x00")
	binary.BigEndian.PutUint32(header[8:12], 2)
	binary.BigEndian.PutUint32(header[12:16], 2)
	for _, tt := range []struct {
		name, rel, data  string
		wantErr          bool
		visits, findings int
	}{
		{"gzip", "a.gz", private, false, 1, 1},
		{"gzip magic", "a.bin", private, false, 1, 1},
		{"second member", "a.gz", safe + private, false, 2, 1},
		{"pack", "a.pack", string(header) + safe + private, false, 2, 1},
		{"catalog ID", "a.pack", string(header) + safe + id, false, 2, 1},
		{"owned fixture ID", "testdata/a.gz", id, false, 1, 0},
		{"captured fixture ID", "internal/lwc/compile/testdata/native.json.gz", id, false, 1, 1},
		{"ZIP resource", "a.resource", archive, false, 4, 1},
		{"ZIP extension", "a.zip", archive, false, 4, 1},
		{"truncated ZIP", "a.resource", archive[:len(archive)-8], true, 0, 0},
		{"truncated", "a.gz", private[:len(private)-4], true, 0, 0},
		{"trailing garbage", "a.gz", safe + "bad", true, 1, 0},
		{"bad header", "a.pack", "bad", true, 0, 0},
		{"missing member", "a.pack", string(header) + safe, true, 1, 0},
		{"binary", "a.png", "\x00image", false, 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			visits, findings := 0, 0
			err := scanPublicAsset(tt.rel, tt.data, func(_ string, text string) {
				visits++
				findings += len(publicDataFindings(tt.rel, text))
			})
			if (err != nil) != tt.wantErr || visits != tt.visits || findings != tt.findings {
				t.Fatalf("error=%v visits=%d findings=%d; want error=%v visits=%d findings=%d", err, visits, findings, tt.wantErr, tt.visits, tt.findings)
			}
		})
	}
}
