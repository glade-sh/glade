package oracle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/open-aer/oaer/internal/capability"
)

const OrchestratorSchemaVersion = 1

type SurfaceStatus string

const (
	SurfaceUnknown           SurfaceStatus = "unknown"
	SurfaceInventoryOnly     SurfaceStatus = "inventory_only"
	SurfaceCompileShapeKnown SurfaceStatus = "compile_shape_known"
	SurfaceRuntimeShapeKnown SurfaceStatus = "runtime_shape_known"
	SurfaceSalesforceSeen    SurfaceStatus = "salesforce_observed"
	SurfaceOAERMatched       SurfaceStatus = "oaer_matched"
	SurfaceOAERUnsupported   SurfaceStatus = "oaer_unsupported"
	SurfaceEnvRequired       SurfaceStatus = "env_required"
	SurfaceManualReview      SurfaceStatus = "manual_review"
)

type Surface struct {
	SurfaceID  string        `json:"surfaceId"`
	Area       string        `json:"area"`
	Type       string        `json:"type"`
	Member     string        `json:"member,omitempty"`
	Kind       string        `json:"kind"`
	Static     bool          `json:"static,omitempty"`
	ReturnType string        `json:"returnType,omitempty"`
	Parameters []string      `json:"parameters,omitempty"`
	Status     SurfaceStatus `json:"status"`
	Sources    []string      `json:"sources,omitempty"`
}

type Inventory struct {
	SchemaVersion int       `json:"schemaVersion"`
	Target        string    `json:"target"`
	GeneratedAt   time.Time `json:"generatedAt"`
	Surfaces      []Surface `json:"surfaces"`
}

type DomainValue struct {
	ID      string `json:"id"`
	Apex    string `json:"apex"`
	Meaning string `json:"meaning,omitempty"`
}

type Domains struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Target        string                   `json:"target"`
	GeneratedAt   time.Time                `json:"generatedAt"`
	Values        map[string][]DomainValue `json:"values"`
}

type ProbeSpec struct {
	ProbeID      string   `json:"probeId"`
	SurfaceID    string   `json:"surfaceId"`
	Area         string   `json:"area"`
	Type         string   `json:"type"`
	Member       string   `json:"member,omitempty"`
	Kind         string   `json:"kind"`
	Mode         string   `json:"mode"`
	Parameters   []string `json:"parameters,omitempty"`
	DomainIDs    []string `json:"domainIds,omitempty"`
	GeneratedCls string   `json:"generatedClass"`
	MethodName   string   `json:"methodName"`
}

type ProbeManifest struct {
	SchemaVersion int         `json:"schemaVersion"`
	Target        string      `json:"target"`
	GeneratedAt   time.Time   `json:"generatedAt"`
	Area          string      `json:"area,omitempty"`
	Specs         []ProbeSpec `json:"specs"`
}

type WorkItem struct {
	ID             string   `json:"id"`
	ProbeID        string   `json:"probeId"`
	SurfaceID      string   `json:"surfaceId"`
	Area           string   `json:"area"`
	Shard          int      `json:"shard"`
	GeneratedClass string   `json:"generatedClass"`
	MethodName     string   `json:"methodName"`
	Status         string   `json:"status"`
	Attempts       int      `json:"attempts"`
	Artifacts      []string `json:"artifacts,omitempty"`
}

type WorkQueue struct {
	SchemaVersion int        `json:"schemaVersion"`
	Target        string     `json:"target"`
	GeneratedAt   time.Time  `json:"generatedAt"`
	Area          string     `json:"area,omitempty"`
	Items         []WorkItem `json:"items"`
}

type OracleCoverage struct {
	SchemaVersion int            `json:"schemaVersion"`
	Target        string         `json:"target"`
	GeneratedAt   time.Time      `json:"generatedAt"`
	ByStatus      map[string]int `json:"byStatus"`
	ByArea        map[string]int `json:"byArea"`
	TotalSurfaces int            `json:"totalSurfaces"`
}

type LedgerRow struct {
	Timestamp string `json:"timestamp"`
	RunID     string `json:"runId"`
	Step      string `json:"step"`
	Shard     int    `json:"shard,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

func BuildInventory(stubRoot string) (Inventory, error) {
	contracts, err := capability.BuildStubContractReport(stubRoot)
	if err != nil {
		return Inventory{}, err
	}
	surfaces := make([]Surface, 0, len(contracts.Entries))
	for _, e := range contracts.Entries {
		surfaceID := e.Type
		if strings.TrimSpace(e.Member) != "" {
			surfaceID = fmt.Sprintf("%s.%s(%s)", e.Type, e.Member, strings.Join(e.Parameters, ","))
		}
		status := SurfaceCompileShapeKnown
		switch e.Mode {
		case capability.StubContractOrgDiff:
			status = SurfaceRuntimeShapeKnown
		case capability.StubContractLocalOnly:
			status = SurfaceOAERUnsupported
		case capability.StubContractPassiveDTO:
			status = SurfaceRuntimeShapeKnown
		case capability.StubContractCompileShape:
			status = SurfaceCompileShapeKnown
		}
		surfaces = append(surfaces, Surface{
			SurfaceID:  surfaceID,
			Area:       surfaceArea(e.Type),
			Type:       e.Type,
			Member:     e.Member,
			Kind:       e.Kind,
			Static:     e.Static,
			ReturnType: e.ReturnType,
			Parameters: append([]string(nil), e.Parameters...),
			Status:     status,
			Sources:    append([]string{"stub-contracts"}, e.Evidence...),
		})
	}
	sort.Slice(surfaces, func(i, j int) bool { return surfaces[i].SurfaceID < surfaces[j].SurfaceID })
	return Inventory{
		SchemaVersion: OrchestratorSchemaVersion,
		Target:        "apex oracle inventory",
		GeneratedAt:   time.Now().UTC(),
		Surfaces:      surfaces,
	}, nil
}

func BuildDomains() Domains {
	return Domains{
		SchemaVersion: OrchestratorSchemaVersion,
		Target:        "apex oracle parameter domains",
		GeneratedAt:   time.Now().UTC(),
		Values: map[string][]DomainValue{
			"String": {
				{ID: "null", Apex: "null", Meaning: "null input"},
				{ID: "empty", Apex: "''", Meaning: "empty string"},
				{ID: "ascii_short", Apex: "'abcdef'", Meaning: "short literal"},
				{ID: "numeric", Apex: "'123.45'", Meaning: "numeric text"},
			},
			"Integer": {
				{ID: "null", Apex: "null"},
				{ID: "zero", Apex: "0"},
				{ID: "one", Apex: "1"},
				{ID: "negative_one", Apex: "-1"},
			},
			"Boolean": {
				{ID: "null", Apex: "null"},
				{ID: "true", Apex: "true"},
				{ID: "false", Apex: "false"},
			},
			"Object": {
				{ID: "null", Apex: "null"},
			},
		},
	}
}

func BuildManifest(inv Inventory, area string, limit int) ProbeManifest {
	specs := make([]ProbeSpec, 0)
	for i, s := range inv.Surfaces {
		if area != "" && s.Area != area {
			continue
		}
		if s.Kind != "method" && s.Kind != "constructor" && s.Kind != "property" {
			continue
		}
		token := strings.ReplaceAll(s.Area, ".", "_")
		probeID := fmt.Sprintf("%s.%04d", token, i+1)
		cls := fmt.Sprintf("OAER_Oracle_%s_%04d", token, i+1)
		mode := "compile-shape"
		switch s.Status {
		case SurfaceRuntimeShapeKnown:
			mode = "org-diff"
		case SurfaceOAERUnsupported:
			mode = "local-contract"
		case SurfaceEnvRequired:
			mode = "env-required"
		}
		specs = append(specs, ProbeSpec{
			ProbeID:      probeID,
			SurfaceID:    s.SurfaceID,
			Area:         s.Area,
			Type:         s.Type,
			Member:       s.Member,
			Kind:         s.Kind,
			Mode:         mode,
			Parameters:   append([]string(nil), s.Parameters...),
			GeneratedCls: cls,
			MethodName:   fmt.Sprintf("probe_%04d", i+1),
		})
		if limit > 0 && len(specs) >= limit {
			break
		}
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].ProbeID < specs[j].ProbeID })
	return ProbeManifest{
		SchemaVersion: OrchestratorSchemaVersion,
		Target:        "apex oracle probe manifest",
		GeneratedAt:   time.Now().UTC(),
		Area:          area,
		Specs:         specs,
	}
}

func BuildWorkQueue(manifest ProbeManifest, shardCount int) WorkQueue {
	if shardCount <= 0 {
		shardCount = 1
	}
	items := make([]WorkItem, 0, len(manifest.Specs))
	for i, s := range manifest.Specs {
		items = append(items, WorkItem{
			ID:             fmt.Sprintf("work.%04d", i+1),
			ProbeID:        s.ProbeID,
			SurfaceID:      s.SurfaceID,
			Area:           s.Area,
			Shard:          i % shardCount,
			GeneratedClass: s.GeneratedCls,
			MethodName:     s.MethodName,
			Status:         "planned",
		})
	}
	return WorkQueue{
		SchemaVersion: OrchestratorSchemaVersion,
		Target:        "apex oracle work queue",
		GeneratedAt:   time.Now().UTC(),
		Area:          manifest.Area,
		Items:         items,
	}
}

func GenerateApex(queue WorkQueue, runDir string) error {
	projectDir := filepath.Join(runDir, "generated", "sfdx")
	classesDir := filepath.Join(projectDir, "force-app", "main", "default", "classes")
	if err := os.MkdirAll(classesDir, 0o755); err != nil {
		return err
	}
	projectFile := filepath.Join(projectDir, "sfdx-project.json")
	projectJSON := `{
  "packageDirectories": [
    {
      "path": "force-app",
      "default": true
    }
  ],
  "namespace": "",
  "sourceApiVersion": "61.0"
}
`
	if err := os.WriteFile(projectFile, []byte(projectJSON), 0o644); err != nil {
		return err
	}
	for _, item := range queue.Items {
		content := fmt.Sprintf("@IsTest\npublic class %s {\n    @IsTest\n    public static void %s() {\n        Map<String, Object> payload = new Map<String, Object>();\n        payload.put('probeId', '%s');\n        payload.put('surfaceId', '%s');\n        payload.put('area', '%s');\n        try {\n            payload.put('status', 'generated');\n            payload.put('result', null);\n            payload.put('exceptionType', null);\n            payload.put('exceptionMessage', null);\n        } catch (Exception ex) {\n            payload.put('status', 'exception');\n            payload.put('result', null);\n            payload.put('exceptionType', ex.getTypeName());\n            payload.put('exceptionMessage', ex.getMessage());\n        }\n        System.debug(LoggingLevel.ERROR, 'OAER_ORACLE ' + JSON.serialize(payload));\n    }\n}\n", item.GeneratedClass, item.MethodName, item.ProbeID, escapeApex(item.SurfaceID), escapeApex(item.Area))
		clsPath := filepath.Join(classesDir, item.GeneratedClass+".cls")
		metaPath := clsPath + "-meta.xml"
		if err := os.WriteFile(clsPath, []byte(content), 0o644); err != nil {
			return err
		}
		meta := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>61.0</apiVersion><status>Active</status></ApexClass>\n"
		if err := os.WriteFile(metaPath, []byte(meta), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func GenerateScripts(queue WorkQueue, runID, runsDir, targetOrg, outDir string, shardCount int) error {
	if shardCount <= 0 {
		shardCount = 1
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for shard := 0; shard < shardCount; shard++ {
		script := fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\nRUN_ID=%q\nRUNS_DIR=%q\nTARGET_ORG=%q\nSHARD=%d\ngo run ./cmd/oaer compat oracle run-salesforce --run-id \"$RUN_ID\" --runs-dir \"$RUNS_DIR\" --target-org \"$TARGET_ORG\" --shard \"$SHARD\" --fetch-logs --log-limit 200\nGO_RUN_OAER=${GO_RUN_OAER:-1}\nif [ \"$GO_RUN_OAER\" = \"1\" ]; then\n  go run ./cmd/oaer compat oracle run-oaer --run-id \"$RUN_ID\" --runs-dir \"$RUNS_DIR\" --shard \"$SHARD\"\n  go run ./cmd/oaer compat oracle diff --run-id \"$RUN_ID\" --runs-dir \"$RUNS_DIR\" --shard \"$SHARD\"\nfi\n", runID, runsDir, targetOrg, shard)
		path := filepath.Join(outDir, fmt.Sprintf("06-run-shard-%03d.sh", shard))
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			return err
		}
	}
	resume := "#!/usr/bin/env bash\nset -euo pipefail\ngo run ./cmd/oaer compat oracle resume \"$@\"\n"
	if err := os.WriteFile(filepath.Join(outDir, "resume-failed.sh"), []byte(resume), 0o755); err != nil {
		return err
	}
	promote := fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\ngo run ./cmd/oaer compat oracle promote --run-id %q --runs-dir %q\n", runID, runsDir)
	if err := os.WriteFile(filepath.Join(outDir, "promote-passing.sh"), []byte(promote), 0o755); err != nil {
		return err
	}
	nightly := fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\nfor f in %q/06-run-shard-*.sh; do bash \"$f\"; done\n", outDir)
	if err := os.WriteFile(filepath.Join(outDir, "nightly-full.sh"), []byte(nightly), 0o755); err != nil {
		return err
	}
	nextAgent := fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\ngo run ./cmd/oaer compat oracle next --run-id %q --runs-dir %q --limit 25 --json\n", runID, runsDir)
	if err := os.WriteFile(filepath.Join(outDir, "next-agent-batch.sh"), []byte(nextAgent), 0o755); err != nil {
		return err
	}
	return nil
}

func BuildCoverage(inv Inventory) OracleCoverage {
	byStatus := map[string]int{}
	byArea := map[string]int{}
	for _, s := range inv.Surfaces {
		byStatus[string(s.Status)]++
		byArea[s.Area]++
	}
	return OracleCoverage{
		SchemaVersion: OrchestratorSchemaVersion,
		Target:        "apex oracle coverage",
		GeneratedAt:   time.Now().UTC(),
		ByStatus:      byStatus,
		ByArea:        byArea,
		TotalSurfaces: len(inv.Surfaces),
	}
}

func NextItems(queue WorkQueue, limit int) []WorkItem {
	pending := make([]WorkItem, 0)
	for _, item := range queue.Items {
		if item.Status == "planned" || item.Status == "blocked_infra" || item.Status == "generated" || item.Status == "incomplete" {
			pending = append(pending, item)
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].Shard == pending[j].Shard {
			return pending[i].ID < pending[j].ID
		}
		return pending[i].Shard < pending[j].Shard
	})
	if limit > 0 && len(pending) > limit {
		pending = pending[:limit]
	}
	return pending
}

func WriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func AppendLedger(path string, row LedgerRow) error {
	if row.Timestamp == "" {
		row.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(row)
}

func ReadLedger(path string) ([]LedgerRow, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	rows := make([]LedgerRow, 0, 128)
	dec := json.NewDecoder(bufio.NewReader(file))
	for {
		var row LedgerRow
		err := dec.Decode(&row)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func QueueForShard(queue WorkQueue, shard int, area string) WorkQueue {
	items := make([]WorkItem, 0, len(queue.Items))
	for _, item := range queue.Items {
		if item.Shard != shard {
			continue
		}
		if area != "" && item.Area != area {
			continue
		}
		items = append(items, item)
	}
	queue.Items = items
	return queue
}

func ClassNames(queue WorkQueue) []string {
	seen := map[string]struct{}{}
	names := make([]string, 0, len(queue.Items))
	for _, item := range queue.Items {
		if _, ok := seen[item.GeneratedClass]; ok {
			continue
		}
		seen[item.GeneratedClass] = struct{}{}
		names = append(names, item.GeneratedClass)
	}
	sort.Strings(names)
	return names
}

func escapeApex(in string) string {
	in = strings.ReplaceAll(in, "\\", "\\\\")
	in = strings.ReplaceAll(in, "'", "\\'")
	return in
}

func surfaceArea(typeName string) string {
	l := strings.ToLower(typeName)
	switch {
	case strings.HasPrefix(l, "system"), strings.HasPrefix(l, "string"), strings.HasPrefix(l, "math"), strings.HasPrefix(l, "date"), strings.HasPrefix(l, "datetime"):
		return "stdlib.core"
	case strings.HasPrefix(l, "database"), strings.HasPrefix(l, "schema"):
		return "stdlib.platform"
	case strings.Contains(l, "sobject"):
		return "sobjects"
	case strings.HasPrefix(l, "test"):
		return "tests"
	default:
		return "stdlib.misc"
	}
}
