//go:build ignore

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
)

type declScope struct {
	Name      string
	Kind      string
	Start     int
	End       int
	StartLine int
	EndLine   int
}

type occurrence struct {
	Project      string `json:"project"`
	File         string `json:"file"`
	Line         int    `json:"line"`
	Column       int    `json:"column"`
	Declaration  string `json:"declaration"`
	DeclKind     string `json:"declarationKind"`
	Category     string `json:"category"`
	Feature      string `json:"feature"`
	Target       string `json:"target,omitempty"`
	Snippet      string `json:"snippet,omitempty"`
	RuntimePath  string `json:"runtimePath"`
	Match        string `json:"match"`
	Offset       int    `json:"offset"`
	RelativePath string `json:"relativePath"`
}

type summary struct {
	TotalFiles               int            `json:"totalFiles"`
	Projects                 int            `json:"projects"`
	TotalOccurrences         int            `json:"totalOccurrences"`
	UniqueRuntimePaths       int            `json:"uniqueRuntimePaths"`
	ByCategory               map[string]int `json:"byCategory"`
	ByFeature                map[string]int `json:"byFeature"`
	ByProject                map[string]int `json:"byProject"`
	ByDeclarationKind        map[string]int `json:"byDeclarationKind"`
	DistinctTargets          int            `json:"distinctTargets"`
	DistinctDeclarations     int            `json:"distinctDeclarations"`
	FilesWithRuntimePaths    int            `json:"filesWithRuntimePaths"`
	FilesWithoutRuntimePaths int            `json:"filesWithoutRuntimePaths"`
	ProjectFilesScanned      map[string]int `json:"projectFilesScanned"`
	ProjectFilesWithPaths    map[string]int `json:"projectFilesWithPaths"`
	ProjectFilesWithoutPaths map[string]int `json:"projectFilesWithoutPaths"`
}

type report struct {
	GeneratedAt   string                     `json:"generatedAt"`
	Root          string                     `json:"root"`
	Input         string                     `json:"input"`
	IncludeStubs  bool                       `json:"includeStubs"`
	FileGlobs     []string                   `json:"fileGlobs"`
	Summary       summary                    `json:"summary"`
	RuntimePaths  []runtimePathSummary       `json:"runtimePaths"`
	Occurrences   []occurrence               `json:"occurrences"`
	ProjectTotals []projectRuntimePathTotals `json:"projectTotals"`
}

type runtimePathSummary struct {
	RuntimePath string `json:"runtimePath"`
	Category    string `json:"category"`
	Feature     string `json:"feature"`
	Target      string `json:"target,omitempty"`
	Count       int    `json:"count"`
}

type projectRuntimePathTotals struct {
	Project               string `json:"project"`
	FilesScanned          int    `json:"filesScanned"`
	FilesWithRuntimePaths int    `json:"filesWithRuntimePaths"`
	TotalOccurrences      int    `json:"totalOccurrences"`
	UniqueRuntimePaths    int    `json:"uniqueRuntimePaths"`
}

type pattern struct {
	Category string
	Feature  string
	Re       *regexp.Regexp
	TargetFn func(string) string
}

var (
	reSOQL        = regexp.MustCompile(`\[(?is:[^\]]*?\bselect\b[^\]]*?)\]`)
	reSOSL        = regexp.MustCompile(`\[(?is:[^\]]*?\bfind\b[^\]]*?)\]`)
	reDMLStmt     = regexp.MustCompile(`(?im)\b(insert|update|delete|upsert|undelete|merge)\s+`) // keyword only
	reDatabase    = regexp.MustCompile(`\bDatabase\.(insert|update|delete|upsert|undelete|merge|query|queryMore|countQuery|getQueryLocator|executeBatch)\s*\(`)
	reSystemRun   = regexp.MustCompile(`\bSystem\.runAs\s*\(`)
	reHttpCall    = regexp.MustCompile(`\bHttp\s+\w+\s*=|\bnew\s+Http\s*\(`)
	reHttpSend    = regexp.MustCompile(`\b\w+\.send\s*\(`)
	reQueueable   = regexp.MustCompile(`\bSystem\.enqueueJob\s*\(`)
	reFutureAnn   = regexp.MustCompile(`(?im)@future(?:\s*\([^\)]*\))?`)
	reBatchable   = regexp.MustCompile(`\bimplements\b[^\n\r\{]*\bDatabase\.Batchable\b`)
	reSchedulable = regexp.MustCompile(`\bimplements\b[^\n\r\{]*\bSchedulable\b`)
	reIterable    = regexp.MustCompile(`\bimplements\b[^\n\r\{]*\bIterable\b`)
	reRestAnn     = regexp.MustCompile(`(?im)@(HttpGet|HttpPost|HttpPut|HttpPatch|HttpDelete|RestResource)\b`)
	reTestAnn     = regexp.MustCompile(`(?im)@(isTest|testSetup)\b`)
	reJSON        = regexp.MustCompile(`\bJSON\.(serialize|deserialize|deserializeUntyped|createGenerator|createParser)\s*\(`)
	rePageRef     = regexp.MustCompile(`\bPageReference\b|\bApexPages\b|\bPage\.[A-Za-z_][A-Za-z0-9_]*`)
	reTriggerCtx  = regexp.MustCompile(`\bTrigger\.(newMap|oldMap|new|old|isBefore|isAfter|isInsert|isUpdate|isDelete|isUndelete|size)\b`)
	reSObjectOp   = regexp.MustCompile(`\b(Schema\.[A-Za-z_][A-Za-z0-9_]*|SObjectType\.[A-Za-z_][A-Za-z0-9_]*|\w+\.getSObjectType\s*\()`)
	reMethodCall  = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_\.]{1,})\s*\(`)
)

func main() {
	var (
		input        string
		outputJSON   string
		outputMD     string
		includeStubs bool
	)
	flag.StringVar(&input, "input", "example-projects", "input root to scan")
	flag.StringVar(&outputJSON, "output", ".glade/runtime-path-inventory.json", "output JSON path")
	flag.StringVar(&outputMD, "markdown", ".glade/runtime-path-inventory.md", "output markdown path")
	flag.BoolVar(&includeStubs, "include-stubs", false, "include example-projects/stubs")
	flag.Parse()

	root, err := os.Getwd()
	if err != nil {
		die("resolve working directory", err)
	}
	inputAbs := filepath.Join(root, input)
	if filepath.IsAbs(input) {
		inputAbs = input
	}
	st, err := os.Stat(inputAbs)
	if err != nil || !st.IsDir() {
		die("input directory", fmt.Errorf("%s is not a readable directory", inputAbs))
	}

	files, err := collectApexFiles(inputAbs, includeStubs)
	if err != nil {
		die("collect apex files", err)
	}
	if len(files) == 0 {
		die("scan apex files", fmt.Errorf("no .cls or .trigger files found under %s", inputAbs))
	}

	parser := apexast.NewParser()
	patterns := buildPatterns()

	occurrences := make([]occurrence, 0, len(files)*8)
	projectFilesScanned := map[string]int{}
	projectFilesWithPaths := map[string]int{}
	projectOccurrenceCounts := map[string]int{}
	projectRuntimePathSet := map[string]map[string]struct{}{}
	filesWithPathsSet := map[string]struct{}{}

	for _, filePath := range files {
		sourceBytes, err := os.ReadFile(filePath)
		if err != nil {
			die("read apex file", err)
		}
		source := string(sourceBytes)
		rel, _ := filepath.Rel(root, filePath)
		project := projectName(inputAbs, filePath)
		projectFilesScanned[project]++

		parsed := parser.ParseSource(filePath, source)
		scopes := declarationScopes(parsed)
		lineStarts := lineStartOffsets(source)

		fileOcc := scanSource(project, rel, source, scopes, lineStarts, patterns)
		if len(fileOcc) > 0 {
			projectFilesWithPaths[project]++
			filesWithPathsSet[rel] = struct{}{}
			if _, ok := projectRuntimePathSet[project]; !ok {
				projectRuntimePathSet[project] = map[string]struct{}{}
			}
			for _, oc := range fileOcc {
				projectOccurrenceCounts[project]++
				projectRuntimePathSet[project][oc.RuntimePath] = struct{}{}
			}
		}
		occurrences = append(occurrences, fileOcc...)
	}

	sort.Slice(occurrences, func(i, j int) bool {
		a, b := occurrences[i], occurrences[j]
		if a.Project != b.Project {
			return a.Project < b.Project
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Feature != b.Feature {
			return a.Feature < b.Feature
		}
		return a.Target < b.Target
	})

	rpCount := map[string]int{}
	rpMeta := map[string]runtimePathSummary{}
	byCategory := map[string]int{}
	byFeature := map[string]int{}
	byProject := map[string]int{}
	byDeclKind := map[string]int{}
	targetSet := map[string]struct{}{}
	declSet := map[string]struct{}{}

	for _, oc := range occurrences {
		rpCount[oc.RuntimePath]++
		if _, ok := rpMeta[oc.RuntimePath]; !ok {
			rpMeta[oc.RuntimePath] = runtimePathSummary{
				RuntimePath: oc.RuntimePath,
				Category:    oc.Category,
				Feature:     oc.Feature,
				Target:      oc.Target,
			}
		}
		byCategory[oc.Category]++
		byFeature[oc.Feature]++
		byProject[oc.Project]++
		byDeclKind[oc.DeclKind]++
		if oc.Target != "" {
			targetSet[oc.Target] = struct{}{}
		}
		declSet[oc.Declaration] = struct{}{}
	}

	runtimePaths := make([]runtimePathSummary, 0, len(rpCount))
	for rp, count := range rpCount {
		entry := rpMeta[rp]
		entry.Count = count
		runtimePaths = append(runtimePaths, entry)
	}
	sort.Slice(runtimePaths, func(i, j int) bool {
		if runtimePaths[i].Count != runtimePaths[j].Count {
			return runtimePaths[i].Count > runtimePaths[j].Count
		}
		return runtimePaths[i].RuntimePath < runtimePaths[j].RuntimePath
	})

	projects := sortedKeys(projectFilesScanned)
	projectTotals := make([]projectRuntimePathTotals, 0, len(projects))
	for _, project := range projects {
		fScanned := projectFilesScanned[project]
		fWith := projectFilesWithPaths[project]
		projectTotals = append(projectTotals, projectRuntimePathTotals{
			Project:               project,
			FilesScanned:          fScanned,
			FilesWithRuntimePaths: fWith,
			TotalOccurrences:      projectOccurrenceCounts[project],
			UniqueRuntimePaths:    len(projectRuntimePathSet[project]),
		})
	}

	projectFilesWithoutPaths := map[string]int{}
	for p, scanned := range projectFilesScanned {
		projectFilesWithoutPaths[p] = scanned - projectFilesWithPaths[p]
	}

	rep := report{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		Root:         root,
		Input:        inputAbs,
		IncludeStubs: includeStubs,
		FileGlobs:    []string{"**/*.cls", "**/*.trigger"},
		Summary: summary{
			TotalFiles:               len(files),
			Projects:                 len(projects),
			TotalOccurrences:         len(occurrences),
			UniqueRuntimePaths:       len(runtimePaths),
			ByCategory:               byCategory,
			ByFeature:                byFeature,
			ByProject:                byProject,
			ByDeclarationKind:        byDeclKind,
			DistinctTargets:          len(targetSet),
			DistinctDeclarations:     len(declSet),
			FilesWithRuntimePaths:    len(filesWithPathsSet),
			FilesWithoutRuntimePaths: len(files) - len(filesWithPathsSet),
			ProjectFilesScanned:      projectFilesScanned,
			ProjectFilesWithPaths:    projectFilesWithPaths,
			ProjectFilesWithoutPaths: projectFilesWithoutPaths,
		},
		RuntimePaths:  runtimePaths,
		Occurrences:   occurrences,
		ProjectTotals: projectTotals,
	}

	if err := writeJSON(outputJSON, rep); err != nil {
		die("write JSON output", err)
	}
	if err := writeMarkdown(outputMD, rep); err != nil {
		die("write markdown output", err)
	}

	fmt.Printf("wrote inventory JSON: %s\n", outputJSON)
	fmt.Printf("wrote inventory Markdown: %s\n", outputMD)
	fmt.Printf("files=%d projects=%d occurrences=%d runtimePaths=%d\n",
		rep.Summary.TotalFiles,
		rep.Summary.Projects,
		rep.Summary.TotalOccurrences,
		rep.Summary.UniqueRuntimePaths,
	)
}

func buildPatterns() []pattern {
	return []pattern{
		{Category: "query", Feature: "soql.literal", Re: reSOQL, TargetFn: shortenMatchTarget(72)},
		{Category: "query", Feature: "sosl.literal", Re: reSOSL, TargetFn: shortenMatchTarget(72)},
		{Category: "dml", Feature: "statement.keyword", Re: reDMLStmt, TargetFn: func(m string) string { return lowerGroupOne(reDMLStmt, m) }},
		{Category: "dml", Feature: "database.method", Re: reDatabase, TargetFn: func(m string) string { return "Database." + strings.ToLower(groupOne(reDatabase, m)) }},
		{Category: "security", Feature: "system.runAs", Re: reSystemRun, TargetFn: fixedTarget("System.runAs")},
		{Category: "http", Feature: "http.client", Re: reHttpCall, TargetFn: fixedTarget("Http")},
		{Category: "http", Feature: "http.send", Re: reHttpSend, TargetFn: extractCallTarget},
		{Category: "async", Feature: "system.enqueueJob", Re: reQueueable, TargetFn: fixedTarget("System.enqueueJob")},
		{Category: "async", Feature: "annotation.future", Re: reFutureAnn, TargetFn: fixedTarget("@future")},
		{Category: "async", Feature: "implements.batchable", Re: reBatchable, TargetFn: fixedTarget("Database.Batchable")},
		{Category: "async", Feature: "implements.schedulable", Re: reSchedulable, TargetFn: fixedTarget("Schedulable")},
		{Category: "async", Feature: "implements.iterable", Re: reIterable, TargetFn: fixedTarget("Iterable")},
		{Category: "rest", Feature: "annotation.rest", Re: reRestAnn, TargetFn: captureAnnotationTarget},
		{Category: "test", Feature: "annotation.test", Re: reTestAnn, TargetFn: captureAnnotationTarget},
		{Category: "serialization", Feature: "json.method", Re: reJSON, TargetFn: extractCallTarget},
		{Category: "ui", Feature: "pageref.apexpages", Re: rePageRef, TargetFn: trimSpace},
		{Category: "trigger", Feature: "trigger.context", Re: reTriggerCtx, TargetFn: trimSpace},
		{Category: "schema", Feature: "sobject.schema", Re: reSObjectOp, TargetFn: trimSpace},
		{Category: "call", Feature: "method.call", Re: reMethodCall, TargetFn: extractCallTarget},
	}
}

func scanSource(project, relPath, source string, scopes []declScope, lineStarts []int, patterns []pattern) []occurrence {
	out := make([]occurrence, 0, 64)
	seen := map[string]struct{}{}
	scanText := maskCommentsAndStrings(source)
	for _, p := range patterns {
		matches := p.Re.FindAllStringIndex(scanText, -1)
		for _, idx := range matches {
			start, end := idx[0], idx[1]
			match := source[start:end]
			target := ""
			if p.TargetFn != nil {
				target = cleanTarget(p.TargetFn(match))
			}
			if p.Feature == "method.call" {
				if !keepMethodCallTarget(target) {
					continue
				}
			}
			line, col := offsetToLineCol(lineStarts, start)
			scope := nearestScope(scopes, start)
			declName := scope.Name
			declKind := scope.Kind
			runtimePath := p.Feature
			if target != "" {
				runtimePath += "::" + target
			}
			if declName != "" {
				runtimePath = declName + " -> " + runtimePath
			}
			key := fmt.Sprintf("%s|%s|%d|%d|%s|%s", relPath, p.Feature, line, col, target, declName)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			snippet := lineSnippetAt(source, lineStarts, line)
			out = append(out, occurrence{
				Project:      project,
				File:         relPath,
				RelativePath: relPath,
				Line:         line,
				Column:       col,
				Declaration:  declName,
				DeclKind:     declKind,
				Category:     p.Category,
				Feature:      p.Feature,
				Target:       target,
				Snippet:      snippet,
				RuntimePath:  runtimePath,
				Match:        strings.TrimSpace(match),
				Offset:       start,
			})
		}
	}
	return out
}

func maskCommentsAndStrings(source string) string {
	b := []byte(source)
	out := make([]byte, len(b))
	copy(out, b)
	const (
		stateCode = iota
		stateLineComment
		stateBlockComment
		stateSingleQuote
		stateDoubleQuote
	)
	state := stateCode
	for i := 0; i < len(b); i++ {
		ch := b[i]
		next := byte(0)
		if i+1 < len(b) {
			next = b[i+1]
		}
		switch state {
		case stateCode:
			if ch == '/' && next == '/' {
				out[i], out[i+1] = ' ', ' '
				i++
				state = stateLineComment
				continue
			}
			if ch == '/' && next == '*' {
				out[i], out[i+1] = ' ', ' '
				i++
				state = stateBlockComment
				continue
			}
			if ch == '\'' {
				out[i] = ' '
				state = stateSingleQuote
				continue
			}
			if ch == '"' {
				out[i] = ' '
				state = stateDoubleQuote
				continue
			}
		case stateLineComment:
			if ch == '\n' {
				state = stateCode
			} else {
				out[i] = ' '
			}
		case stateBlockComment:
			out[i] = ' '
			if ch == '*' && next == '/' {
				out[i+1] = ' '
				i++
				state = stateCode
			}
		case stateSingleQuote:
			out[i] = ' '
			if ch == '\\' && i+1 < len(b) {
				out[i+1] = ' '
				i++
				continue
			}
			if ch == '\'' {
				state = stateCode
			}
		case stateDoubleQuote:
			out[i] = ' '
			if ch == '\\' && i+1 < len(b) {
				out[i+1] = ' '
				i++
				continue
			}
			if ch == '"' {
				state = stateCode
			}
		}
	}
	return string(out)
}

func collectApexFiles(inputAbs string, includeStubs bool) ([]string, error) {
	files := make([]string, 0, 1024)
	err := filepath.WalkDir(inputAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if strings.HasPrefix(base, ".") && path != inputAbs {
				return filepath.SkipDir
			}
			if !includeStubs && strings.Contains(path, string(filepath.Separator)+"stubs") {
				return filepath.SkipDir
			}
			if base == "node_modules" || base == "dist" || base == "build" || base == "target" || base == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".cls" || ext == ".trigger" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func declarationScopes(file apexast.File) []declScope {
	scopes := make([]declScope, 0, 64)
	apexast.WalkFile(file, apexast.VisitorFunc(func(decl apexast.Declaration) bool {
		name := decl.Name
		if name == "" {
			name = string(decl.Kind)
		}
		if decl.Kind == apexast.DeclarationTrigger && decl.ObjectName != "" {
			name = name + "(" + decl.ObjectName + ")"
		}
		scopes = append(scopes, declScope{
			Name:      name,
			Kind:      string(decl.Kind),
			Start:     decl.Range.Start.Offset,
			End:       decl.Range.End.Offset,
			StartLine: decl.Range.Start.Line,
			EndLine:   decl.Range.End.Line,
		})
		return true
	}))
	sort.Slice(scopes, func(i, j int) bool {
		if scopes[i].Start != scopes[j].Start {
			return scopes[i].Start < scopes[j].Start
		}
		return scopes[i].End > scopes[j].End
	})
	return scopes
}

func nearestScope(scopes []declScope, offset int) declScope {
	best := declScope{Name: "<file>", Kind: "file", Start: 0, End: 1 << 30}
	bestWidth := best.End - best.Start
	for _, s := range scopes {
		if offset < s.Start || offset > s.End {
			continue
		}
		w := s.End - s.Start
		if w <= bestWidth {
			best = s
			bestWidth = w
		}
	}
	return best
}

func lineStartOffsets(source string) []int {
	starts := []int{0}
	for i, b := range []byte(source) {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func offsetToLineCol(starts []int, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	i := sort.Search(len(starts), func(i int) bool { return starts[i] > offset }) - 1
	if i < 0 {
		i = 0
	}
	line := i + 1
	col := offset - starts[i] + 1
	if col < 1 {
		col = 1
	}
	return line, col
}

func lineSnippetAt(source string, starts []int, line int) string {
	if line < 1 || line > len(starts) {
		return ""
	}
	start := starts[line-1]
	end := len(source)
	if line < len(starts) {
		end = starts[line] - 1
	}
	if start >= 0 && end >= start && end <= len(source) {
		return strings.TrimSpace(source[start:end])
	}
	return ""
}

func cleanTarget(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "(")
	s = strings.TrimPrefix(s, "@")
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

func keepMethodCallTarget(target string) bool {
	if target == "" {
		return false
	}
	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "if") || strings.HasPrefix(lower, "for") || strings.HasPrefix(lower, "while") || strings.HasPrefix(lower, "switch") || strings.HasPrefix(lower, "catch") || strings.HasPrefix(lower, "return") || strings.HasPrefix(lower, "new") {
		return false
	}
	parts := strings.Split(target, ".")
	last := strings.ToLower(parts[len(parts)-1])
	if last == "if" || last == "for" || last == "while" || last == "switch" {
		return false
	}
	return true
}

func writeJSON(path string, rep report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

func writeMarkdown(path string, rep report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	fmt.Fprintf(w, "# Runtime Path Inventory\n\n")
	fmt.Fprintf(w, "Generated: `%s`\n\n", rep.GeneratedAt)
	fmt.Fprintf(w, "Input: `%s`\n\n", rep.Input)
	fmt.Fprintf(w, "Files scanned: `%d`  \\n", rep.Summary.TotalFiles)
	fmt.Fprintf(w, "Projects: `%d`  \\n", rep.Summary.Projects)
	fmt.Fprintf(w, "Occurrences: `%d`  \\n", rep.Summary.TotalOccurrences)
	fmt.Fprintf(w, "Unique runtime paths: `%d`\n\n", rep.Summary.UniqueRuntimePaths)

	fmt.Fprintf(w, "## Per Project\n\n")
	fmt.Fprintf(w, "| Project | Files | Files With Paths | Occurrences | Unique Runtime Paths |\n")
	fmt.Fprintf(w, "|---|---:|---:|---:|---:|\n")
	for _, row := range rep.ProjectTotals {
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d |\n", row.Project, row.FilesScanned, row.FilesWithRuntimePaths, row.TotalOccurrences, row.UniqueRuntimePaths)
	}

	fmt.Fprintf(w, "\n## Top Runtime Paths\n\n")
	fmt.Fprintf(w, "| Runtime Path | Category | Feature | Count |\n")
	fmt.Fprintf(w, "|---|---|---|---:|\n")
	limit := 200
	if len(rep.RuntimePaths) < limit {
		limit = len(rep.RuntimePaths)
	}
	for i := 0; i < limit; i++ {
		rp := rep.RuntimePaths[i]
		fmt.Fprintf(w, "| %s | %s | %s | %d |\n", escapePipe(rp.RuntimePath), rp.Category, rp.Feature, rp.Count)
	}
	if len(rep.RuntimePaths) > limit {
		fmt.Fprintf(w, "\n_Only top %d runtime paths shown. See JSON for full inventory._\n", limit)
	}
	return nil
}

func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func projectName(inputAbs, filePath string) string {
	rel, err := filepath.Rel(inputAbs, filePath)
	if err != nil {
		return "<unknown>"
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 0 || parts[0] == "." || parts[0] == "" {
		return "<root>"
	}
	return parts[0]
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func lowerGroupOne(re *regexp.Regexp, full string) string {
	m := re.FindStringSubmatch(full)
	if len(m) < 2 {
		return strings.ToLower(strings.TrimSpace(full))
	}
	return strings.ToLower(m[1])
}

func groupOne(re *regexp.Regexp, full string) string {
	m := re.FindStringSubmatch(full)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func fixedTarget(target string) func(string) string {
	return func(_ string) string { return target }
}

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

func extractCallTarget(s string) string {
	i := strings.Index(s, "(")
	if i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func captureAnnotationTarget(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "@") {
		s = s[1:]
	}
	if i := strings.Index(s, "("); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func shortenMatchTarget(limit int) func(string) string {
	return func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.Join(strings.Fields(s), " ")
		if len(s) > limit {
			s = s[:limit]
		}
		return s
	}
}

func die(step string, err error) {
	fmt.Fprintf(os.Stderr, "runtime-inventory: %s: %v\n", step, err)
	os.Exit(1)
}
