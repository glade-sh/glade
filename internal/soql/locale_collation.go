package soql

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/storage"
	"golang.org/x/text/collate"
	"golang.org/x/text/collate/build"
	"golang.org/x/text/unicode/norm"
)

//go:generate python3 collation_generate.py

// Only public UTF-8 text data is embedded. ICU is a generation dependency;
// query execution uses Go and loads immutable tables on first use per profile.
//
//go:embed collationdata/*.txt collationdata/*.json
var localeCollationData embed.FS

type collationProfileOptions struct {
	CaseFirst   int    `json:"caseFirst"`
	Alternate   int    `json:"alternate"`
	VariableTop uint16 `json:"variableTop"`
}

type collationCatalog struct {
	locales  map[string]string
	profiles map[string]func() *elementTable
}

var localeCollationCatalog = sync.OnceValue(func() *collationCatalog {
	var manifest struct {
		Locales  map[string]string                  `json:"locales"`
		Profiles map[string]collationProfileOptions `json:"profiles"`
	}
	raw, err := localeCollationData.ReadFile("collationdata/manifest.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		panic(err)
	}
	catalog := &collationCatalog{locales: map[string]string{}, profiles: map[string]func() *elementTable{}}
	for locale, profile := range manifest.Locales {
		catalog.locales[strings.ToLower(locale)] = profile
	}
	for name, options := range manifest.Profiles {
		catalog.profiles[name] = sync.OnceValue(func() *elementTable {
			table := decodeElementTable(name + ".json")
			table.options = options
			table.parent = baseElementTable()
			return table
		})
	}
	return catalog
})

// The builder's public root-table API works with the current CLDR root data.
// Its Tailoring.Build API is unimplemented, so tailorings use owned generated
// element tables below rather than falling back to its older locale tables.
var rootCollationFactory = sync.OnceValue(func() func(bool) *collate.Collator {
	raw := collationDataBytes("cldr_root.txt")
	builder := build.NewBuilder()
	elements := regexp.MustCompile(`\[([.*])([0-9A-F]+)\.([0-9A-F]+)\.([0-9A-F]+)\]`)
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' || line[0] == '@' {
			continue
		}
		text, weights, ok := strings.Cut(line, ";")
		if !ok {
			panic("invalid root weight row")
		}
		var runes []rune
		for _, token := range strings.Fields(text) {
			n, err := strconv.ParseInt(token, 16, 32)
			if err != nil {
				panic(err)
			}
			runes = append(runes, rune(n))
		}
		var ces [][]int
		var variables []int
		for i, match := range elements.FindAllStringSubmatch(weights, -1) {
			ce := make([]int, 3)
			for j := range ce {
				n, err := strconv.ParseInt(match[j+2], 16, 32)
				if err != nil {
					panic(err)
				}
				ce[j] = int(n)
			}
			// UCA's single FFFD primary is beyond the builder's ordinary-primary
			// encoding. Its public Add contract allows weight renumbering: encode
			// this last regular primary with the builder's highest-weight escape.
			// FFFE itself has primary 1 in CLDR and remains at the beginning.
			if ce[0] == 0xfffd {
				ce[0] = 0xffff
			}
			ces = append(ces, ce)
			if match[1] == "*" {
				variables = append(variables, i)
			}
		}
		if err := builder.Add(runes, ces, variables); err != nil {
			panic(fmt.Sprintf("root table row %s: %v", text, err))
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	table, err := builder.Build()
	if err != nil {
		panic(err)
	}
	return func(secondary bool) *collate.Collator {
		if secondary {
			return collate.NewFromTable(table, collate.IgnoreCase)
		}
		return collate.NewFromTable(table)
	}
})

var baseElementTable = sync.OnceValue(func() *elementTable { return decodeElementTable("elements_root.json") })

type elementRange struct {
	first, last rune
	elements    []uint32
	stride      []int64
}

type elementContraction struct {
	text     string
	elements []uint32
}

type elementTable struct {
	ranges       []elementRange
	contractions map[rune][]elementContraction
	reorder      map[uint16]uint16
	options      collationProfileOptions
	parent       *elementTable
}

func collationDataBytes(name string) []byte {
	raw, err := localeCollationData.ReadFile("collationdata/" + name)
	if err != nil {
		panic(err)
	}
	return raw
}

func decodeElementTable(name string) *elementTable {
	var wire struct {
		Format string
		Ranges []struct {
			First, Last rune
			Elements    []uint32
			Stride      []int64
		}
		Contractions []struct {
			Text     string
			Elements []uint32
		}
		Reorder [][2]uint16
	}
	if err := json.Unmarshal(collationDataBytes(name), &wire); err != nil {
		panic(err)
	}
	if wire.Format != "GLDC2" || len(wire.Ranges) > 0x110000 || len(wire.Contractions) > 1<<20 || len(wire.Reorder) > 65536 {
		panic("invalid collation table")
	}
	table := &elementTable{contractions: map[rune][]elementContraction{}, reorder: map[uint16]uint16{}}
	for _, entry := range wire.Ranges {
		if entry.First < 0 || entry.Last > utf8.MaxRune || entry.Last < entry.First || len(entry.Elements) > 128 || len(entry.Elements) != len(entry.Stride) || (len(table.ranges) > 0 && entry.First <= table.ranges[len(table.ranges)-1].last) {
			panic("invalid collation range")
		}
		table.ranges = append(table.ranges, elementRange{first: entry.First, last: entry.Last, elements: entry.Elements, stride: entry.Stride})
	}
	for _, entry := range wire.Contractions {
		first, size := utf8.DecodeRuneInString(entry.Text)
		if size == 0 || !utf8.ValidString(entry.Text) || len(entry.Elements) > 128 {
			panic("invalid collation contraction")
		}
		table.contractions[first] = append(table.contractions[first], elementContraction{text: entry.Text, elements: entry.Elements})
	}
	for first, entries := range table.contractions {
		sort.Slice(entries, func(i, j int) bool { return len(entries[i].text) > len(entries[j].text) })
		table.contractions[first] = entries
	}
	for _, pair := range wire.Reorder {
		table.reorder[pair[0]] = pair[1]
	}
	return table
}

func (table *elementTable) appendScalar(dst []uint32, r rune) ([]uint32, bool) {
	i := sort.Search(len(table.ranges), func(i int) bool { return table.ranges[i].last >= r })
	if i == len(table.ranges) || r < table.ranges[i].first {
		if table.parent != nil {
			return table.parent.appendScalar(dst, r)
		}
		return dst, false
	}
	entry := table.ranges[i]
	offset := int64(r - entry.first)
	for j, ce := range entry.elements {
		dst = append(dst, uint32(int64(ce)+offset*entry.stride[j]))
	}
	return dst, true
}

type elementKey struct{ primary, secondary, tertiary []byte }

func (table *elementTable) key(text string) elementKey {
	text = norm.NFD.String(text)
	var ces []uint32
	for len(text) > 0 {
		first, size := utf8.DecodeRuneInString(text)
		matched := false
		for _, c := range table.contractions[first] {
			if strings.HasPrefix(text, c.text) {
				ces = append(ces, c.elements...)
				text = text[len(c.text):]
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		var ok bool
		ces, ok = table.appendScalar(ces, first)
		if !ok {
			panic("scalar missing from generated collation table")
		}
		text = text[size:]
	}
	var key elementKey
	shifted := false
	for _, ce := range ces {
		primary := uint16(ce >> 16)
		secondary := byte(ce >> 8)
		tertiary := byte(ce)
		continuation := tertiary&0xc0 == 0xc0
		if table.options.Alternate == 20 {
			if primary != 0 && !continuation {
				shifted = primary <= table.options.VariableTop
			}
			if shifted {
				continue
			}
		}
		if primary != 0 {
			if !continuation {
				if reordered, ok := table.reorder[primary]; ok {
					primary = reordered
				}
			}
			key.primary = append(key.primary, byte(primary>>8), byte(primary))
		}
		if secondary != 0 {
			key.secondary = append(key.secondary, secondary)
		}
		if continuation {
			continue
		}
		weight := tertiary & 0x3f
		if weight != 0 {
			switch table.options.CaseFirst {
			case 25:
				weight += 0x80 - (tertiary & 0xc0)
			case 24:
				weight += tertiary & 0xc0
			}
			key.tertiary = append(key.tertiary, weight)
		}
	}
	return key
}

// textComparator is private to one query execution. The cached tables above
// are immutable; collator buffers and memoized string keys are never shared
// with another execution or stored in the parsed-query cache.
type textComparator struct {
	locale                       string
	english                      bool
	root                         bool
	loadElements                 func() *elementTable
	rootOrder, rootRange         *collate.Collator
	rootBuffer                   *collate.Buffer
	rootOrderKeys, rootRangeKeys map[string]string
	elements                     *elementTable
	elementKeys                  map[string]elementKey
}

func newTextComparator(locale string) *textComparator {
	if locale == "" {
		return nil
	}
	if englishTextLocale(locale) {
		return &textComparator{locale: locale, english: true}
	}
	catalog := localeCollationCatalog()
	profile := catalog.locales[strings.ToLower(locale)]
	switch profile {
	case "english":
		return &textComparator{locale: locale, english: true}
	case "root":
		return &textComparator{locale: locale, root: true}
	case "", "legacy":
		return nil
	}
	load := catalog.profiles[profile]
	if load == nil {
		panic("missing locale profile")
	}
	return &textComparator{locale: locale, loadElements: load}
}

func (c *textComparator) rootKey(text string, secondary bool) string {
	keys := &c.rootOrderKeys
	if secondary {
		if c.rootRange == nil {
			c.rootRange = rootCollationFactory()(true)
		}
	} else if c.rootOrder == nil {
		c.rootOrder = rootCollationFactory()(false)
	}
	collator := c.rootOrder
	if secondary {
		keys = &c.rootRangeKeys
		collator = c.rootRange
	}
	if *keys == nil {
		*keys = map[string]string{}
	}
	if key, ok := (*keys)[text]; ok {
		return key
	}
	if c.rootBuffer == nil {
		c.rootBuffer = &collate.Buffer{}
	}
	c.rootBuffer.Reset()
	key := string(collator.KeyFromString(c.rootBuffer, text))
	(*keys)[text] = key
	return key
}

func (c *textComparator) elementKey(text string) elementKey {
	if c.elements == nil {
		c.elements = c.loadElements()
	}
	if c.elementKeys == nil {
		c.elementKeys = map[string]elementKey{}
	}
	if key, ok := c.elementKeys[text]; ok {
		return key
	}
	key := c.elements.key(text)
	c.elementKeys[text] = key
	return key
}

func (c *textComparator) compare(a, b string, secondary bool) int {
	if c.english {
		return compareSOQLTextCollated(a, b)
	}
	if c.root {
		return strings.Compare(c.rootKey(a, secondary), c.rootKey(b, secondary))
	}
	left, right := c.elementKey(a), c.elementKey(b)
	if cmp := bytes.Compare(left.primary, right.primary); cmp != 0 {
		return cmp
	}
	if cmp := bytes.Compare(left.secondary, right.secondary); cmp != 0 {
		return cmp
	}
	if secondary {
		return 0
	}
	return bytes.Compare(left.tertiary, right.tertiary)
}

func (c *textComparator) groupKey(text string) string {
	if c.english {
		return textGroupKey(text)
	}
	if c.root {
		return base64.RawStdEncoding.EncodeToString([]byte(c.rootKey(text, false)))
	}
	key := c.elementKey(text)
	var encoded []byte
	for _, level := range [][]byte{key.primary, key.secondary, key.tertiary} {
		encoded = binary.AppendUvarint(encoded, uint64(len(level)))
		encoded = append(encoded, level...)
	}
	return base64.RawStdEncoding.EncodeToString(encoded)
}

func compareValuesForCollation(left, right storage.Value, c *textComparator, secondary bool) int {
	if c != nil && left.Kind == storage.ValueString && right.Kind == storage.ValueString {
		return c.compare(left.String, right.String, secondary)
	}
	return compareValues(left, right)
}

func (context ExecutionContext) textComparatorForQuery() *textComparator {
	if context.textComparator != nil && context.textComparator.locale == context.Locale {
		return context.textComparator
	}
	return newTextComparator(context.Locale)
}
