package soql

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/storage"
)

func TestLocaleCollationExecutionOwnsMutableKeys(t *testing.T) {
	first, second := newTextComparator("sv_SE"), newTextComparator("sv_SE")
	first.elementKey("ä")
	second.elementKey("ä")
	if first == second || first.elements != second.elements {
		t.Fatal("queries must own separate state over one immutable table")
	}
	key := first.elementKey("ä")
	key.primary[0] ^= 0xff
	if second.compare("z", "ä", false) >= 0 || newTextComparator("sv_SE").compare("z", "ä", false) >= 0 {
		t.Fatal("returned query key mutated shared table state")
	}
	root, other := newTextComparator("de_DE"), newTextComparator("de_DE")
	root.rootKey("ä", false)
	other.rootKey("ä", false)
	root.rootKey("ä", true)
	other.rootKey("ä", true)
	if root.rootOrder == other.rootOrder || root.rootRange == other.rootRange {
		t.Fatal("root collator buffers shared across queries")
	}
	held := root.rootKey("ä", false)
	root.rootKey("z", false)
	if root.rootKey("ä", false) != held || other.rootKey("ä", false) != held {
		t.Fatal("root key aliases a reused collation buffer")
	}
	for _, locale := range []string{"de_DE", "sv_SE", "ja_JP"} {
		c := newTextComparator(locale)
		if c.compare("é", "e\u0301", false) != 0 || c.groupKey("é") != c.groupKey("e\u0301") {
			t.Error("canonical class changed", locale)
		}
		if strings.ContainsRune(c.groupKey("é"), 0) {
			t.Error("group key contains the composite-key separator", locale)
		}
	}
}

func TestLocaleCollationStrengthAndRootBoundary(t *testing.T) {
	c := newTextComparator("de_DE")
	if c.compare("a", "Ａ", true) != 0 || c.compare("a", "Ａ", false) == 0 {
		t.Fatal("secondary and tertiary strengths are not distinct")
	}
	if c.compare("\ufffd", "😀", false) <= 0 || c.compare("\ufffd", "\U0010ffff", false) <= 0 || c.compare("\ufffe", "_", false) >= 0 {
		t.Fatal("root highest-weight escape lost the replacement/noncharacter boundary")
	}
}

func TestLocaleCollationFallbackPolicy(t *testing.T) {
	for _, locale := range []string{"lv_LV", "he_IL", "id_ID", "nb_NO", "nn_NO", "fo_FO", "gl_ES", "mn_MN", "si_LK", "es_ES_TRADITIONAL", "ja_JP_TRADITIONAL", "zz_ZZ", ""} {
		if newTextComparator(locale) != nil {
			t.Error("uncaptured/unfitting locale unexpectedly enabled", locale)
		}
	}
	// Existing en_* behavior remains intact for unlisted English region keys.
	if c := newTextComparator("en_ZZ"); c == nil || !c.english {
		t.Fatal("existing English-region fallback changed")
	}
}

func TestLocaleCollationGeneratedDataIntegrity(t *testing.T) {
	raw, err := localeCollationData.ReadFile("collationdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		GeneratorSHA256    string            `json:"generatorSHA256"`
		RootWeightsSHA256  string            `json:"rootWeightsSHA256"`
		BaseElementsSHA256 string            `json:"baseElementsSHA256"`
		Locales            map[string]string `json:"locales"`
		Profiles           map[string]struct {
			SHA256    string `json:"sha256"`
			Conflicts int    `json:"reorderConflicts"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Locales) != 96 {
		t.Fatalf("locale policy entries %d, want 96", len(m.Locales))
	}
	check := func(name, expected string, raw []byte) {
		t.Helper()
		actual := sha256.Sum256(raw)
		if hex.EncodeToString(actual[:]) != expected {
			t.Error("generated checksum mismatch", name)
		}
	}
	generator, err := os.ReadFile("collation_generate.py")
	if err != nil {
		t.Fatal(err)
	}
	check("generator", m.GeneratorSHA256, generator)
	check("CLDR root", m.RootWeightsSHA256, collationDataBytes("cldr_root.txt"))
	raw, err = localeCollationData.ReadFile("collationdata/elements_root.json")
	if err != nil {
		t.Fatal(err)
	}
	check("element root", m.BaseElementsSHA256, raw)
	for name, p := range m.Profiles {
		if p.Conflicts != 0 {
			t.Error("ambiguous primary reordering", name)
		}
		raw, err := localeCollationData.ReadFile("collationdata/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		check(name, p.SHA256, raw)
	}
}

func TestLocaleCollationLoadsTablesOnlyForText(t *testing.T) {
	for _, locale := range []string{"de_DE", "sv_SE"} {
		comparator := newTextComparator(locale)
		compareValuesForCollation(storage.IntegerValue(1), storage.IntegerValue(2), comparator, false)
		if comparator.elements != nil || comparator.rootOrder != nil || comparator.rootRange != nil {
			t.Error("non-text comparison loaded locale tables", locale)
		}
		comparator.compare("ä", "z", false)
		if comparator.elements == nil && comparator.rootOrder == nil {
			t.Error("text comparison did not load its table", locale)
		}
	}
}

func TestLocaleCollationDoesNotStampParsedCaches(t *testing.T) {
	org := collationOrg([]string{"ä", "z"}, "Account", map[string]bool{"Name": true})
	const text = "SELECT Name,MIN(BillingCity) m FROM Account GROUP BY Name ORDER BY MIN(BillingCity)"
	parsed, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	cache := &ExecutionCache{}
	for _, locale := range []string{"de_DE", "sv_SE", "en_US", "lv_LV", "de_DE"} {
		if _, err := ExecuteWithCacheAndContext(org, parsed, cache, ExecutionContext{Locale: locale}); err != nil {
			t.Fatal(err)
		}
		if parsed.textComparator != nil || parsed.textLocale != "" {
			t.Fatal("execution state escaped into the caller query")
		}
		for _, aggregate := range parsed.Aggregates {
			if aggregate.textComparator != nil {
				t.Fatal("aggregate slice retained execution state")
			}
		}
		for _, order := range parsed.Order {
			if order.textComparator != nil {
				t.Fatal("order slice retained execution state")
			}
		}
		again, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		if again.textComparator != nil || again.textLocale != "" {
			t.Fatal("execution state escaped into parsed-query cache")
		}
	}
}

// Every committed payload is scanner-readable text. Hashes above fail on drift
// in any table or the generator, even on hosts without the pinned ICU library.
func TestLocaleCollationGeneratedTablesAreText(t *testing.T) {
	entries, err := localeCollationData.ReadDir("collationdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		raw := collationDataBytes(entry.Name())
		if !utf8.Valid(raw) || strings.ContainsRune(string(raw), 0) {
			t.Errorf("generated payload is not UTF-8 text: %s", entry.Name())
		}
		if strings.HasSuffix(entry.Name(), ".json") && !json.Valid(raw) {
			t.Errorf("generated table is not inspectable JSON: %s", entry.Name())
		}
	}
}

// The checksum test above runs everywhere. This test also regenerates from
// public APIs on hosts with the pinned generation dependency (not the runtime).
func TestLocaleCollationGeneratorReproducesTables(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("not run: Python 3 generation dependency unavailable")
	}
	probe := exec.Command(python, "-I", "-c", "import runpy; m=runpy.run_path('collation_generate.py'); m['ICU']('/usr/lib/libicucore.dylib','')")
	if output, err := probe.CombinedOutput(); err != nil {
		t.Skipf("not run: pinned ICU 78.1 / CLDR 48 generation dependency unavailable: %v\n%s", err, output)
	}
	command := exec.Command(python, "-I", "collation_generate.py", "--check")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("collation generator drift: %v\n%s", err, output)
	}
}
