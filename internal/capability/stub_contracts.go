package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const StubContractsSchemaVersion = 1

type StubContractMode string

const (
	StubContractOrgDiff      StubContractMode = "org-diff"
	StubContractLocalOnly    StubContractMode = "local-contract"
	StubContractPassiveDTO   StubContractMode = "passive-dto"
	StubContractCompileShape StubContractMode = "compile-shape"
)

type StubContractReport struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Target        string              `json:"target"`
	Source        StubContractSource  `json:"source"`
	Totals        StubContractTotals  `json:"totals"`
	Entries       []StubContractEntry `json:"entries"`
}

type StubContractSource struct {
	BehaviorEntries  int `json:"behaviorEntries"`
	SystemStubTypes  int `json:"systemStubTypes"`
	SObjectStubTypes int `json:"sobjectStubTypes"`
}

type StubContractTotals struct {
	Entries   int            `json:"entries"`
	Types     int            `json:"types"`
	Members   int            `json:"members"`
	ByMode    map[string]int `json:"byMode"`
	ByStatus  map[string]int `json:"byStatus"`
	WithProbe int            `json:"withProbe"`
}

type StubContractEntry struct {
	ID                string             `json:"id"`
	Type              string             `json:"type"`
	Member            string             `json:"member,omitempty"`
	Kind              string             `json:"kind"`
	Static            bool               `json:"static,omitempty"`
	ReturnType        string             `json:"returnType,omitempty"`
	Parameters        []string           `json:"parameters,omitempty"`
	Status            StubBehaviorStatus `json:"status"`
	Mode              StubContractMode   `json:"mode"`
	ProbeID           string             `json:"probeId,omitempty"`
	Owner             string             `json:"owner"`
	RequiresOrgProbe  bool               `json:"requiresOrgProbe,omitempty"`
	UnsupportedReason string             `json:"unsupportedReason,omitempty"`
	Normalization     string             `json:"normalization,omitempty"`
	FailureShape      string             `json:"failureShape,omitempty"`
	OddityRisk        string             `json:"oddityRisk,omitempty"`
	EdgeTags          []string           `json:"edgeTags,omitempty"`
	Evidence          []string           `json:"evidence,omitempty"`
	Notes             string             `json:"notes,omitempty"`
}

type StubContractProbeSpec struct {
	ID               string           `json:"id"`
	ContractID       string           `json:"contractId"`
	Type             string           `json:"type"`
	Member           string           `json:"member,omitempty"`
	Kind             string           `json:"kind"`
	Static           bool             `json:"static,omitempty"`
	ReturnType       string           `json:"returnType,omitempty"`
	Parameters       []string         `json:"parameters,omitempty"`
	Mode             StubContractMode `json:"mode"`
	Tier             string           `json:"tier"`
	CanBatch         bool             `json:"canBatch"`
	RequiresOrgProbe bool             `json:"requiresOrgProbe"`
}

func BuildStubContractReport(sourceRoot string) (StubContractReport, error) {
	behavior := BuildStubBehaviorReport()
	inventory, err := BuildStubInventoryReport(sourceRoot)
	if err != nil {
		return StubContractReport{}, err
	}
	report := StubContractReport{
		SchemaVersion: StubContractsSchemaVersion,
		Target:        "generated Apex stub behavioral contracts",
		Source: StubContractSource{
			BehaviorEntries:  behavior.Totals.Entries,
			SystemStubTypes:  inventory.Source.SystemStubClasses,
			SObjectStubTypes: inventory.Source.SObjectStubClasses,
		},
		Entries: make([]StubContractEntry, 0, len(behavior.Entries)),
	}
	for _, entry := range behavior.Entries {
		contract := buildStubContractEntry(entry)
		report.Entries = append(report.Entries, contract)
	}
	sort.Slice(report.Entries, func(i, j int) bool {
		return report.Entries[i].ID < report.Entries[j].ID
	})
	report.Totals = countStubContractTotals(report.Entries)
	return report, nil
}

func WriteStubContractsJSON(w io.Writer, report StubContractReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteStubContractsMarkdown(w io.Writer, report StubContractReport) error {
	if _, err := fmt.Fprintln(w, "# Stub Contracts"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\nTarget: %s\n", report.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\n- Entries: %d\n", report.Totals.Entries); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Types: %d\n", report.Totals.Types); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Members: %d\n", report.Totals.Members); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- With probes: %d\n", report.Totals.WithProbe); err != nil {
		return err
	}
	for _, mode := range []StubContractMode{StubContractOrgDiff, StubContractLocalOnly, StubContractPassiveDTO, StubContractCompileShape} {
		if _, err := fmt.Fprintf(w, "- %s: %d\n", mode, report.Totals.ByMode[string(mode)]); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, "\n| ID | Mode | Status | Probe | Owner |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- | --- | --- |"); err != nil {
		return err
	}
	for _, entry := range report.Entries {
		if _, err := fmt.Fprintf(w, "| `%s` | `%s` | `%s` | `%s` | %s |\n", entry.ID, entry.Mode, entry.Status, entry.ProbeID, entry.Owner); err != nil {
			return err
		}
	}
	return nil
}

func WriteStubContractProbeManifestJSON(w io.Writer, specs []StubContractProbeSpec) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(specs)
}

func BuildStubContractProbeManifest(report StubContractReport, tier string) []StubContractProbeSpec {
	specs := make([]StubContractProbeSpec, 0, len(report.Entries))
	for _, entry := range report.Entries {
		if !stubContractProbeExecutable(entry) {
			continue
		}
		probeID := entry.ProbeID
		if probeID == "" {
			probeID = stubContractProbeID(StubBehaviorEntry{Type: entry.Type, Member: entry.Member})
		}
		spec := StubContractProbeSpec{
			ID:               probeID,
			ContractID:       entry.ID,
			Type:             entry.Type,
			Member:           entry.Member,
			Kind:             entry.Kind,
			Static:           entry.Static,
			ReturnType:       entry.ReturnType,
			Parameters:       append([]string(nil), entry.Parameters...),
			Mode:             entry.Mode,
			Tier:             stubContractTier(entry),
			CanBatch:         entry.Mode != StubContractLocalOnly,
			RequiresOrgProbe: entry.Mode == StubContractOrgDiff,
		}
		if tier != "" && tier != "full" && !stubContractTierMatch(tier, spec.Tier) {
			continue
		}
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].ID < specs[j].ID
	})
	return specs
}

func stubContractProbeExecutable(entry StubContractEntry) bool {
	if entry.Member == "" {
		return false
	}
	switch strings.ToLower(entry.Kind) {
	case "method", "constructor", "property":
		return true
	default:
		return false
	}
}

func buildStubContractEntry(entry StubBehaviorEntry) StubContractEntry {
	mode := classifyStubContractMode(entry)
	contract := StubContractEntry{
		ID:         entry.ID,
		Type:       entry.Type,
		Member:     entry.Member,
		Kind:       entry.Kind,
		Static:     entry.Static,
		ReturnType: entry.ReturnType,
		Parameters: append([]string(nil), entry.Parameters...),
		Status:     entry.Status,
		Mode:       mode,
		Owner:      classifyStubContractOwner(entry),
		Evidence:   append([]string(nil), entry.Evidence...),
		Notes:      entry.Notes,
	}
	contract.Normalization = stubContractNormalization(entry)
	contract.FailureShape = stubContractFailureShape(entry)
	contract.OddityRisk, contract.EdgeTags = stubContractOddityProfile(entry)
	if entry.Status == StubBehaviorUnsupported {
		contract.UnsupportedReason = firstNonEmpty(entry.Notes, "explicit unsupported platform surface")
	}
	if mode == StubContractOrgDiff {
		contract.ProbeID = stubContractProbeID(entry)
		contract.RequiresOrgProbe = true
	}
	return contract
}

func classifyStubContractMode(entry StubBehaviorEntry) StubContractMode {
	switch entry.Status {
	case StubBehaviorUnsupported:
		return StubContractLocalOnly
	case StubBehaviorUnknown:
		return StubContractCompileShape
	}
	switch entry.Kind {
	case "constructor", "property":
		return StubContractPassiveDTO
	}
	if entry.Kind == "method" && entry.Status == StubBehaviorImplemented && orgDiffCandidate(entry) {
		return StubContractOrgDiff
	}
	if entry.Kind == "method" && entry.Status == StubBehaviorPassiveDefault {
		return StubContractPassiveDTO
	}
	return StubContractCompileShape
}

func classifyStubContractOwner(entry StubBehaviorEntry) string {
	lowerType := strings.ToLower(entry.Type)
	switch {
	case strings.HasPrefix(lowerType, "schema"), strings.Contains(lowerType, "sobject"):
		return "metadata-runtime"
	case strings.HasPrefix(lowerType, "database"), strings.HasPrefix(lowerType, "soql"):
		return "data-runtime"
	case strings.HasPrefix(lowerType, "apexpages"), strings.HasPrefix(lowerType, "site"), strings.HasPrefix(lowerType, "network"):
		return "ui-runtime"
	case strings.HasPrefix(lowerType, "test"), strings.HasPrefix(lowerType, "async"), strings.HasPrefix(lowerType, "eventbus"):
		return "test-runtime"
	default:
		return "vm-stdlib"
	}
}

func orgDiffCandidate(entry StubBehaviorEntry) bool {
	if entry.Kind != "method" {
		return false
	}
	if entry.Static && strings.EqualFold(entry.Member, "valueOf") {
		return true
	}
	lowerType := strings.ToLower(entry.Type)
	switch {
	case lowerType == "system", lowerType == "string", lowerType == "math", lowerType == "date", lowerType == "datetime", lowerType == "time":
		return true
	case strings.HasPrefix(lowerType, "json"), strings.HasPrefix(lowerType, "schema"), strings.HasPrefix(lowerType, "encodingutil"):
		return true
	case strings.HasPrefix(lowerType, "pattern"), strings.HasPrefix(lowerType, "matcher"), strings.HasPrefix(lowerType, "crypto"), strings.HasPrefix(lowerType, "blob"):
		return true
	}
	return false
}

func stubContractProbeID(entry StubBehaviorEntry) string {
	var b strings.Builder
	b.WriteString("stub.")
	b.WriteString(normalizeProbeToken(entry.Type))
	if entry.Member != "" {
		b.WriteString(".")
		b.WriteString(normalizeProbeToken(entry.Member))
	}
	return b.String()
}

func stubContractTier(entry StubContractEntry) string {
	if entry.Mode == StubContractLocalOnly {
		return "local"
	}
	lowerType := strings.ToLower(entry.Type)
	if lowerType == "system" || lowerType == "string" || lowerType == "math" || lowerType == "date" || lowerType == "datetime" || strings.HasPrefix(lowerType, "schema") {
		return "smoke"
	}
	if entry.Mode == StubContractOrgDiff {
		return "core"
	}
	return "full"
}

func stubContractTierMatch(requested, actual string) bool {
	switch requested {
	case "core":
		return actual == "core" || actual == "smoke"
	default:
		return requested == actual
	}
}

func stubContractNormalization(entry StubBehaviorEntry) string {
	lowerType := strings.ToLower(entry.Type)
	lowerMember := strings.ToLower(entry.Member)
	switch {
	case lowerType == "string":
		return "normalize unicode/escape semantics and null behavior to Apex observable output"
	case strings.HasPrefix(lowerType, "datetime"), lowerType == "date", lowerType == "time":
		return "normalize timezone and locale-dependent output before diffing"
	case strings.HasPrefix(lowerType, "json"):
		return "normalize map key order and serialized whitespace before diffing"
	case strings.HasPrefix(lowerType, "schema"):
		return "normalize metadata ordering and case-insensitive lookup keys"
	case lowerMember == "equals" || lowerMember == "hashcode":
		return "normalize object identity semantics for deterministic local comparison"
	default:
		return ""
	}
}

func stubContractFailureShape(entry StubBehaviorEntry) string {
	if entry.Status == StubBehaviorUnsupported {
		return "stable unsupported diagnostic"
	}
	if strings.EqualFold(entry.Kind, "constructor") {
		return "constructor exception type and message parity"
	}
	if strings.EqualFold(entry.Kind, "method") {
		return "exception type/message parity, including null and bounds errors"
	}
	return ""
}

func stubContractOddityProfile(entry StubBehaviorEntry) (string, []string) {
	lowerType := strings.ToLower(entry.Type)
	lowerMember := strings.ToLower(entry.Member)
	tags := make([]string, 0, 4)
	risk := ""
	switch {
	case lowerType == "string":
		risk = "high"
		tags = append(tags, "null-handling", "unicode", "regex-dialect", "locale-case-fold")
	case strings.HasPrefix(lowerType, "datetime") || lowerType == "date" || lowerType == "time":
		risk = "high"
		tags = append(tags, "timezone", "dst-boundary", "locale-format")
	case strings.HasPrefix(lowerType, "math") || lowerType == "decimal":
		risk = "medium"
		tags = append(tags, "rounding", "scale", "overflow")
	case strings.HasPrefix(lowerType, "json"):
		risk = "medium"
		tags = append(tags, "number-coercion", "field-order", "null-token")
	case strings.HasPrefix(lowerType, "schema"):
		risk = "high"
		tags = append(tags, "feature-gating", "describe-shape", "namespace-alias")
	}
	if lowerMember == "valueof" || lowerMember == "parse" {
		tags = append(tags, "parse-errors")
		if risk == "" {
			risk = "medium"
		}
	}
	if lowerMember == "split" || lowerMember == "replaceall" {
		tags = append(tags, "regex-compat")
		if risk == "" {
			risk = "high"
		}
	}
	if len(tags) == 0 {
		return "", nil
	}
	return risk, dedupeStrings(tags)
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeProbeToken(in string) string {
	in = strings.TrimSpace(strings.ToLower(in))
	replacer := strings.NewReplacer(
		"(", "-", ")", "", "<", "-", ">", "", ",", "-", ".", "-", " ", "-", "[", "-", "]", "",
	)
	in = replacer.Replace(in)
	for strings.Contains(in, "--") {
		in = strings.ReplaceAll(in, "--", "-")
	}
	return strings.Trim(in, "-")
}

func countStubContractTotals(entries []StubContractEntry) StubContractTotals {
	totals := StubContractTotals{
		Entries:  len(entries),
		ByMode:   map[string]int{},
		ByStatus: map[string]int{},
	}
	types := map[string]struct{}{}
	for _, entry := range entries {
		types[entry.Type] = struct{}{}
		if entry.Member != "" {
			totals.Members++
		}
		totals.ByMode[string(entry.Mode)]++
		totals.ByStatus[string(entry.Status)]++
		if entry.ProbeID != "" {
			totals.WithProbe++
		}
	}
	totals.Types = len(types)
	return totals
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
