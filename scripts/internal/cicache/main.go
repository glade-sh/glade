// Command cicache checks cache invariants in GitHub Actions workflows.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

var (
	fullActionSHA   = regexp.MustCompile(`^[^@\s]+@[0-9a-f]{40}$`)
	goBuildRE       = regexp.MustCompile(`\bgo\s+(?:test|build|run|install)\b`)
	goModDownloadRE = regexp.MustCompile(`\bgo\s+mod\s+download\b`)
	npmPopulateRE   = regexp.MustCompile(`\bnpm\s+(?:ci|install|test|run)\b`)
	goVersionRE     = regexp.MustCompile(`(?:go)?[0-9]+\.[0-9]+(?:\.[0-9]+)?`)
	nodeVersionRE   = regexp.MustCompile(`(?:node)?[0-9]{2}(?:\.[0-9]+(?:\.[0-9]+)?)?`)
	primaryKeyRE    = regexp.MustCompile(`^\$\{\{\s*steps\.([A-Za-z0-9_-]+)\.outputs\.cache-primary-key\s*\}\}$`)
	envRefRE        = regexp.MustCompile(`\$\{\{\s*env\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)
	expressionRE    = regexp.MustCompile(`\$\{\{.*?\}\}`)
	semverRE        = regexp.MustCompile(`\b[0-9]+\.[0-9]+(?:\.[0-9]+)?\b`)
	matrixEqualsRE  = regexp.MustCompile(`(?:matrix\.([A-Za-z_][A-Za-z0-9_-]*)\s*==\s*(?:'([^']*)'|"([^"]*)"|([A-Za-z0-9_.-]+)))|(?:(?:'([^']*)'|"([^"]*)"|([A-Za-z0-9_.-]+))\s*==\s*matrix\.([A-Za-z_][A-Za-z0-9_-]*))`)
)

type workflow struct {
	Jobs map[string]job `yaml:"jobs"`
}

type job struct {
	Steps    []step         `yaml:"steps"`
	Env      map[string]any `yaml:"env"`
	Strategy strategy       `yaml:"strategy"`
}

type strategy struct {
	Matrix map[string]any `yaml:"matrix"`
}

type step struct {
	ID   string         `yaml:"id"`
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	If   any            `yaml:"if"`
	With map[string]any `yaml:"with"`
	Env  map[string]any `yaml:"env"`
}

func main() {
	check := flag.Bool("check", false, "check workflow cache invariants")
	workflowDir := flag.String("workflow-dir", ".github/workflows", "workflow directory")
	evidencePath := flag.String("cache-evidence", "", "optional JSON cache transfer evidence")
	evidenceMode := flag.String("evidence-mode", "structural", "evidence mode: structural or strict")
	flag.Parse()
	if !*check || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: cicache --check [--workflow-dir <dir>]")
		os.Exit(2)
	}

	problems, err := checkWorkflowDir(*workflowDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[ci-cache]", err)
		os.Exit(1)
	}
	switch *evidenceMode {
	case "structural":
		if *evidencePath != "" {
			fmt.Fprintln(os.Stderr, "[ci-cache] --cache-evidence requires --evidence-mode strict")
			os.Exit(2)
		}
	case "strict":
		if *evidencePath == "" {
			fmt.Fprintln(os.Stderr, "[ci-cache] strict evidence mode requires --cache-evidence")
			os.Exit(2)
		}
		data, err := os.ReadFile(*evidencePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "[ci-cache] read cache evidence:", err)
			os.Exit(1)
		}
		identities, err := cacheIdentitiesDir(*workflowDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "[ci-cache] cache identities:", err)
			os.Exit(1)
		}
		evidenceProblems, err := checkStrictEvidence(data, identities)
		if err != nil {
			fmt.Fprintln(os.Stderr, "[ci-cache] cache evidence:", err)
			os.Exit(1)
		}
		problems = append(problems, evidenceProblems...)
	default:
		fmt.Fprintln(os.Stderr, "[ci-cache] --evidence-mode must be structural or strict")
		os.Exit(2)
	}
	for _, problem := range problems {
		fmt.Fprintln(os.Stderr, "[ci-cache]", problem)
	}
	if len(problems) != 0 {
		os.Exit(1)
	}
	if *evidenceMode == "strict" {
		fmt.Println("[ci-cache] strict cache evidence check passed")
	} else {
		fmt.Println("[ci-cache] structural cache workflow invariants passed (no cache value proof)")
	}
}

func checkWorkflowDir(dir string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.y*ml"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no workflow files found in %s", dir)
	}
	sort.Strings(paths)
	var problems []string
	savedKeys := make(map[string]string)
	implicitNPMWriters := make(map[string]string)
	repoRoot, err := repositoryRootForWorkflowDir(dir)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path) // #nosec G304 -- path comes from the caller-owned workflow directory glob above.
		if err != nil {
			return nil, err
		}
		problems = append(problems, checkWorkflowWithOwnership(filepath.Base(path), data, savedKeys, implicitNPMWriters, repoRoot)...)
	}
	sort.Strings(problems)
	return problems, nil
}

func cacheIdentitiesDir(dir string) (map[string]bool, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.y*ml"))
	if err != nil {
		return nil, err
	}
	identities := make(map[string]bool)
	repoRoot, err := repositoryRootForWorkflowDir(dir)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path) // #nosec G304 -- path comes from the caller-owned workflow directory glob above.
		if err != nil {
			return nil, err
		}
		var source workflow
		if err := yaml.Unmarshal(data, &source); err != nil {
			return nil, fmt.Errorf("%s: parse workflow YAML: %w", filepath.Base(path), err)
		}
		resolved, err := cacheIdentitiesResolved(filepath.Base(path), source, repoRoot)
		if err != nil {
			return nil, fmt.Errorf("%s: dependency identity: %w", filepath.Base(path), err)
		}
		for identity := range resolved {
			identities[identity] = true
		}
	}
	return identities, nil
}

func cacheIdentities(workflowName string, source workflow) map[string]bool {
	identities, err := cacheIdentitiesConfigured(workflowName, source, "")
	if err != nil {
		panic(err)
	}
	return identities
}

func cacheIdentitiesResolved(workflowName string, source workflow, repoRoot string) (map[string]bool, error) {
	return cacheIdentitiesConfigured(workflowName, source, repoRoot)
}

func cacheIdentitiesConfigured(workflowName string, source workflow, repoRoot string) (map[string]bool, error) {
	identities := make(map[string]bool)
	for jobName, candidateJob := range source.Jobs {
		primaryKeys := make(map[string]string)
		toolchains := jobToolchains(candidateJob)
		producer := matrixProducerContext(candidateJob)
		checkoutPrefix := checkoutWorkspacePrefix(candidateJob)
		for _, candidate := range candidateJob.Steps {
			operation := classifyCache(candidate)
			if operation != noCache {
				key := resolvePrimaryKey(withValue(candidate, "key"), primaryKeys)
				paths := normalizePaths(withValue(candidate, "path"), candidateJob.Env, candidate.Env)
				if len(paths) != 0 {
					dependencies, err := dependencyFilesForCache(candidate, key, repoRoot, checkoutPrefix, candidateJob.Env)
					if err != nil {
						return nil, fmt.Errorf("%s: %w", jobName, err)
					}
					identity := cacheConfigurationIdentity(workflowName, jobName, cacheIdentityConfiguration{
						Operation:    operation.String(),
						Uses:         strings.TrimSpace(candidate.Uses),
						If:           normalizeCondition(candidate.If),
						Paths:        paths,
						PrimaryKey:   key,
						RestoreKeys:  normalizeArtifactPaths(withValue(candidate, "restore-keys")),
						Dependencies: dependencies,
						Toolchain:    toolchainConfiguration{Go: toolchains[goCache], Node: toolchains[npmCache]},
						Producer:     producer,
					})
					identities[identity] = true
				}
				if operation.restores() && candidate.ID != "" && key != "" {
					primaryKeys[candidate.ID] = key
				}
			}
			if implicitNPMCachePath(candidate, candidateJob.Env) != "" {
				dependencies, err := dependencyFilesForImplicitNPM(candidate, repoRoot, checkoutPrefix, candidateJob.Env)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", jobName, err)
				}
				identity := cacheConfigurationIdentity(workflowName, jobName, cacheIdentityConfiguration{
					Operation:    "implicit-npm",
					Uses:         strings.TrimSpace(candidate.Uses),
					If:           normalizeCondition(candidate.If),
					Paths:        normalizePaths(withValue(candidate, "cache-dependency-path"), candidateJob.Env, candidate.Env),
					Dependencies: dependencies,
					Toolchain:    toolchainConfiguration{Go: toolchains[goCache], Node: toolchains[npmCache]},
					Producer:     producer,
				})
				identities[identity] = true
			}
		}
	}
	return identities, nil
}

type toolchainConfiguration struct {
	Go   string `json:"go"`
	Node string `json:"node"`
}

type cacheIdentityConfiguration struct {
	Operation    string                   `json:"operation"`
	Uses         string                   `json:"uses"`
	If           string                   `json:"if"`
	Paths        []string                 `json:"paths"`
	PrimaryKey   string                   `json:"primary_key"`
	RestoreKeys  []string                 `json:"restore_keys"`
	Dependencies []dependencyFileIdentity `json:"dependencies"`
	Toolchain    toolchainConfiguration   `json:"toolchain"`
	Producer     string                   `json:"producer"`
}

type dependencyFileIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func cacheConfigurationIdentity(workflowName, jobName string, configuration cacheIdentityConfiguration) string {
	sort.Strings(configuration.Paths)
	encoded, err := json.Marshal(configuration)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(encoded)
	return workflowName + "/" + jobName + "/" + configuration.Operation + "/" + fmt.Sprintf("%x", digest[:])
}

func normalizeCondition(condition any) string {
	if condition == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(condition))
	var normalized strings.Builder
	var quote rune
	escaped := false
	pendingSpace := false
	for _, char := range text {
		if quote != 0 {
			normalized.WriteRune(char)
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			if pendingSpace && normalized.Len() != 0 {
				normalized.WriteByte(' ')
			}
			pendingSpace = false
			quote = char
			normalized.WriteRune(char)
			continue
		}
		if unicode.IsSpace(char) {
			pendingSpace = true
			continue
		}
		if pendingSpace && normalized.Len() != 0 {
			normalized.WriteByte(' ')
		}
		pendingSpace = false
		normalized.WriteRune(char)
	}
	return normalized.String()
}

func repositoryRootForWorkflowDir(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	if filepath.Base(absolute) == "workflows" && filepath.Base(filepath.Dir(absolute)) == ".github" {
		return filepath.Dir(filepath.Dir(absolute)), nil
	}
	return absolute, nil
}

func checkoutWorkspacePrefix(source job) string {
	for _, candidate := range source.Steps {
		if actionName(candidate.Uses) == "actions/checkout" {
			checkoutPath := strings.TrimSpace(withValue(candidate, "path"))
			if checkoutPath == "" {
				return ""
			}
			return strings.Trim(filepath.ToSlash(filepath.Clean(checkoutPath)), "/")
		}
	}
	return ""
}

func dependencyFilesForCache(candidate step, key, repoRoot, checkoutPrefix string, jobEnv map[string]any) ([]dependencyFileIdentity, error) {
	if repoRoot == "" {
		return nil, nil
	}
	patterns, err := literalHashFilePatterns(key + "\n" + withValue(candidate, "restore-keys"))
	if err != nil {
		return nil, err
	}
	return resolveDependencyFiles(repoRoot, checkoutPrefix, patterns, jobEnv, candidate.Env)
}

func dependencyFilesForImplicitNPM(candidate step, repoRoot, checkoutPrefix string, jobEnv map[string]any) ([]dependencyFileIdentity, error) {
	if repoRoot == "" {
		return nil, nil
	}
	patterns := normalizePaths(withValue(candidate, "cache-dependency-path"), jobEnv, candidate.Env)
	if len(patterns) == 0 {
		return nil, fmt.Errorf("implicit npm cache has no dependency files")
	}
	return resolveDependencyFiles(repoRoot, checkoutPrefix, patterns, jobEnv, candidate.Env)
}

func literalHashFilePatterns(value string) ([]string, error) {
	lower := strings.ToLower(value)
	var patterns []string
	for offset := 0; ; {
		relative := strings.Index(lower[offset:], "hashfiles")
		if relative < 0 {
			break
		}
		start := offset + relative
		cursor := start + len("hashfiles")
		for cursor < len(value) && unicode.IsSpace(rune(value[cursor])) {
			cursor++
		}
		if cursor >= len(value) || value[cursor] != '(' {
			return nil, fmt.Errorf("unsupported hashFiles expression")
		}
		bodyStart := cursor + 1
		bodyEnd, err := closingCallParen(value, bodyStart)
		if err != nil {
			return nil, err
		}
		arguments, err := literalStringArguments(value[bodyStart:bodyEnd])
		if err != nil {
			return nil, fmt.Errorf("unsupported hashFiles arguments: %w", err)
		}
		patterns = append(patterns, arguments...)
		offset = bodyEnd + 1
	}
	return patterns, nil
}

func closingCallParen(value string, start int) (int, error) {
	var quote byte
	escaped := false
	for cursor := start; cursor < len(value); cursor++ {
		char := value[cursor]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				if cursor+1 < len(value) && value[cursor+1] == quote {
					cursor++
					continue
				}
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == ')' {
			return cursor, nil
		}
	}
	return 0, fmt.Errorf("unterminated hashFiles expression")
}

func literalStringArguments(value string) ([]string, error) {
	var arguments []string
	for cursor := 0; ; {
		for cursor < len(value) && unicode.IsSpace(rune(value[cursor])) {
			cursor++
		}
		if cursor == len(value) {
			if len(arguments) == 0 {
				return nil, fmt.Errorf("no literal paths")
			}
			return arguments, nil
		}
		quote := value[cursor]
		if quote != '\'' && quote != '"' {
			return nil, fmt.Errorf("path is not a string literal")
		}
		cursor++
		var argument strings.Builder
		closed := false
		for cursor < len(value) {
			char := value[cursor]
			if char == quote {
				if cursor+1 < len(value) && value[cursor+1] == quote {
					argument.WriteByte(quote)
					cursor += 2
					continue
				}
				cursor++
				closed = true
				break
			}
			if char == '\\' && cursor+1 < len(value) {
				cursor++
				char = value[cursor]
			}
			argument.WriteByte(char)
			cursor++
		}
		if !closed || argument.Len() == 0 {
			return nil, fmt.Errorf("invalid literal path")
		}
		arguments = append(arguments, argument.String())
		for cursor < len(value) && unicode.IsSpace(rune(value[cursor])) {
			cursor++
		}
		if cursor == len(value) {
			return arguments, nil
		}
		if value[cursor] != ',' {
			return nil, fmt.Errorf("expected comma between literal paths")
		}
		cursor++
	}
}

func resolveDependencyFiles(repoRoot, checkoutPrefix string, patterns []string, jobEnv, stepEnv map[string]any) ([]dependencyFileIdentity, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	env := mergedEnv(jobEnv, stepEnv)
	files := make(map[string]dependencyFileIdentity)
	for _, rawPattern := range patterns {
		pattern := strings.TrimSpace(envRefRE.ReplaceAllStringFunc(rawPattern, func(match string) string {
			name := envRefRE.FindStringSubmatch(match)[1]
			return env[name]
		}))
		if pattern == "" || strings.Contains(pattern, "${{") || strings.Contains(pattern, "}}") {
			return nil, fmt.Errorf("dependency path %q is dynamic or empty", rawPattern)
		}
		workspacePattern := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(filepath.FromSlash(pattern))), "./")
		if filepath.IsAbs(filepath.FromSlash(workspacePattern)) || workspacePattern == ".." || strings.HasPrefix(workspacePattern, "../") {
			return nil, fmt.Errorf("dependency path %q escapes the repository", rawPattern)
		}
		repoPattern := workspacePattern
		if checkoutPrefix != "" {
			if workspacePattern == checkoutPrefix {
				return nil, fmt.Errorf("dependency path %q resolves to the checkout directory", rawPattern)
			}
			prefix := checkoutPrefix + "/"
			if strings.HasPrefix(workspacePattern, prefix) {
				repoPattern = strings.TrimPrefix(workspacePattern, prefix)
			}
		}
		if strings.Contains(repoPattern, "**") {
			return nil, fmt.Errorf("dependency glob %q uses unsupported recursive matching", rawPattern)
		}
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(repoPattern)))
		if err != nil {
			return nil, fmt.Errorf("dependency glob %q: %w", rawPattern, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("dependency input %q did not match a file", rawPattern)
		}
		for _, match := range matches {
			resolved, err := filepath.EvalSymlinks(match)
			if err != nil {
				return nil, fmt.Errorf("resolve dependency input %q: %w", rawPattern, err)
			}
			relative, err := filepath.Rel(root, resolved)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("dependency input %q resolves outside the repository", rawPattern)
			}
			info, err := os.Stat(resolved)
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("dependency input %q is not a regular file", rawPattern)
			}
			content, err := os.ReadFile(resolved)
			if err != nil {
				return nil, err
			}
			digest := sha256.Sum256(content)
			normalizedPath := filepath.ToSlash(relative)
			files[normalizedPath] = dependencyFileIdentity{
				Path:   normalizedPath,
				SHA256: fmt.Sprintf("%x", digest[:]),
			}
		}
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	identities := make([]dependencyFileIdentity, 0, len(paths))
	for _, path := range paths {
		identities = append(identities, files[path])
	}
	return identities, nil
}

func checkWorkflow(name string, data []byte) []string {
	return checkWorkflowWithOwnership(name, data, make(map[string]string), make(map[string]string), "")
}

func checkWorkflowWithOwnership(name string, data []byte, savedKeys, implicitNPMWriters map[string]string, repoRoot string) []string {
	var source workflow
	if err := yaml.Unmarshal(data, &source); err != nil {
		return []string{fmt.Sprintf("%s: parse workflow YAML: %v", name, err)}
	}
	jobNames := make([]string, 0, len(source.Jobs))
	for jobName := range source.Jobs {
		jobNames = append(jobNames, jobName)
	}
	sort.Strings(jobNames)

	var problems []string
	for _, jobName := range jobNames {
		problems = append(problems, checkJob(name, jobName, source.Jobs[jobName], savedKeys, implicitNPMWriters, repoRoot)...)
	}
	problems = append(problems, checkRequiredUploads(name, source)...)
	return problems
}

func checkJob(workflowName, jobName string, source job, savedKeys, implicitNPMWriters map[string]string, repoRoot string) []string {
	prefix := workflowName + "/" + jobName + ": "
	var problems []string
	checkoutSeen := false
	populatedPaths := make(map[string]bool)
	toolchains := jobToolchains(source)
	matrixRequirements := matrixRequirementsFor(source)
	primaryKeys := make(map[string]string)
	knownPaths := cachePaths(source)

	for _, candidate := range source.Steps {
		if candidate.Run != "" {
			for _, path := range pathsPopulatedByRun(candidate.Run, knownPaths, source.Env, candidate.Env) {
				populatedPaths[path] = true
			}
		}
		if candidate.Uses == "" {
			continue
		}
		if !fullActionSHA.MatchString(candidate.Uses) {
			problems = append(problems, prefix+"action is not pinned to a full SHA: "+candidate.Uses)
		}
		if actionName(candidate.Uses) == "actions/checkout" {
			if checkoutSeen {
				problems = append(problems, prefix+"repeated checkout")
			}
			checkoutSeen = true
		}
		if actionName(candidate.Uses) == "actions/upload-artifact" {
			if withValue(candidate, "name") == "" || withValue(candidate, "path") == "" {
				problems = append(problems, prefix+"upload artifact lacks a name or path")
			}
		}
		if actionName(candidate.Uses) == "actions/setup-node" {
			problems = append(problems, setupNodeCacheProblems(prefix, candidate)...)
			problems = append(problems, implicitNPMMatrixWriterProblems(prefix, candidate, matrixRequirements)...)
			writerIdentity, err := implicitNPMWriterIdentity(candidate, source, repoRoot)
			if err != nil {
				problems = append(problems, prefix+"cannot resolve implicit npm cache ownership: "+err.Error())
			} else if writerIdentity != "" {
				owner := workflowName + "/" + jobName
				if existing, found := implicitNPMWriters[writerIdentity]; found {
					problems = append(problems, prefix+"duplicate implicit npm cache writer (already owned by "+existing+")")
				} else {
					implicitNPMWriters[writerIdentity] = owner
				}
			}
			if path := implicitNPMCachePath(candidate, source.Env); path != "" && populatedPaths[path] {
				problems = append(problems, prefix+"setup-node npm cache restore follows a command that can populate it")
			}
		}
		if actionName(candidate.Uses) == "actions/setup-go" && withValue(candidate, "cache") != "false" {
			problems = append(problems, prefix+"setup-go must set cache: false")
		}

		kind := classifyCache(candidate)
		if kind == noCache {
			continue
		}
		path, key := withValue(candidate, "path"), resolvePrimaryKey(withValue(candidate, "key"), primaryKeys)
		cacheType := cacheKindFor(path, key)
		paths := normalizePaths(path, source.Env, candidate.Env)
		if kind.restores() {
			for _, normalized := range paths {
				for populated := range populatedPaths {
					if pathsOverlap(normalized, populated) {
						problems = append(problems, fmt.Sprintf("%scache restore for %s follows a command that can populate it", prefix, path))
						break
					}
				}
			}
		}
		problems = append(problems, cacheKeyProblems(prefix, cacheType, key, toolchains)...)
		if kind.restores() {
			problems = append(problems, restoreKeyProblems(prefix, key, withValue(candidate, "restore-keys"))...)
		}
		if kind.saves() && key != "" {
			problems = append(problems, matrixKeyProblems(prefix, key, candidate.If, matrixRequirements)...)
			if owner, exists := savedKeys[key]; exists {
				problems = append(problems, prefix+"duplicate immutable cache save key (already saved by "+owner+")")
			}
			savedKeys[key] = workflowName + "/" + jobName
		}
		if kind.restores() && candidate.ID != "" && key != "" {
			primaryKeys[candidate.ID] = key
		}
	}
	return problems
}

type matrixRequirements struct {
	dimensions      []string
	dimensionValues map[string][]any
	includeRows     []map[string]any
}

func matrixRequirementsFor(source job) matrixRequirements {
	requirements := matrixRequirements{dimensionValues: make(map[string][]any)}
	for name, value := range source.Strategy.Matrix {
		switch name {
		case "exclude":
			continue
		case "include":
			rows, _ := value.([]any)
			for _, raw := range rows {
				rowMap, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				row := make(map[string]any, len(rowMap))
				for key, item := range rowMap {
					row[key] = item
				}
				requirements.includeRows = append(requirements.includeRows, row)
			}
		default:
			if values, ok := value.([]any); !ok || len(values) > 1 {
				requirements.dimensions = append(requirements.dimensions, name)
				if ok {
					requirements.dimensionValues[name] = values
				}
			}
		}
	}
	sort.Strings(requirements.dimensions)
	return requirements
}

func matrixKeyProblems(prefix, key string, condition any, requirements matrixRequirements) []string {
	var problems []string
	selectors, selectorIsProven := matrixSelectors(condition)
	if !selectorIsProven {
		selectors = nil
	}
	for _, dimension := range requirements.dimensions {
		if strings.Contains(key, "${{ matrix."+dimension+" }}") {
			continue
		}
		selector, found := selectors[dimension]
		if !found || !selectorProvesSingleStaticDimensionWriter(requirements, dimension, selector) {
			problems = append(problems, prefix+"cache save key lacks varying matrix dimension "+dimension)
		}
	}
	if len(requirements.includeRows) < 2 {
		return problems
	}
	rows, rowsAreProven := selectedMatrixRows(requirements.includeRows, selectors)
	if !rowsAreProven {
		return append(problems, prefix+"cache save key does not uniquely identify matrix include producers")
	}
	if len(rows) == 0 && len(selectors) != 0 {
		return append(problems, prefix+"cache save selector does not match a matrix include producer")
	}
	if len(rows) == 1 {
		return problems
	}
	fields := make(map[string]bool)
	for _, row := range rows {
		for field := range row {
			if strings.Contains(key, "${{ matrix."+field+" }}") {
				fields[field] = true
			}
		}
	}
	fieldNames := make([]string, 0, len(fields))
	for field := range fields {
		fieldNames = append(fieldNames, field)
	}
	sort.Strings(fieldNames)
	seen := make(map[string]bool)
	for _, row := range rows {
		parts := make([]string, 0, len(fieldNames))
		for _, field := range fieldNames {
			parts = append(parts, field+"="+fmt.Sprint(row[field]))
		}
		projection := strings.Join(parts, "\x00")
		if seen[projection] {
			problems = append(problems, prefix+"cache save key does not uniquely identify matrix include producers")
			break
		}
		seen[projection] = true
	}
	return problems
}

func implicitNPMMatrixWriterProblems(prefix string, candidate step, requirements matrixRequirements) []string {
	if !setupNodeCachesNPM(candidate) || (len(requirements.dimensions) == 0 && len(requirements.includeRows) < 2) {
		return nil
	}
	selectors, selectorIsProven := matrixSelectors(candidate.If)
	if !selectorIsProven {
		return []string{prefix + "setup-node npm cache requires a deterministic single matrix writer"}
	}
	for _, dimension := range requirements.dimensions {
		selector, found := selectors[dimension]
		if !found || !selectorProvesSingleStaticDimensionWriter(requirements, dimension, selector) {
			return []string{prefix + "setup-node npm cache requires a deterministic single matrix writer"}
		}
	}
	if len(requirements.includeRows) < 2 {
		return nil
	}
	rows, rowsAreProven := selectedMatrixRows(requirements.includeRows, selectors)
	if rowsAreProven && len(rows) == 1 {
		return nil
	}
	return []string{prefix + "setup-node npm cache requires a deterministic single matrix writer"}
}

func selectorProvesSingleStaticDimensionWriter(requirements matrixRequirements, dimension, selector string) bool {
	values, found := requirements.dimensionValues[dimension]
	if !found || len(values) < 2 || selector == "" {
		return false
	}
	matches := 0
	for _, value := range values {
		stringValue, isString := value.(string)
		if !isString || stringValue == "" {
			return false
		}
		if strings.EqualFold(stringValue, selector) {
			matches++
		}
	}
	return matches == 1
}

func selectedMatrixRows(rows []map[string]any, selectors map[string]string) ([]map[string]any, bool) {
	selectedRows := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		selected := true
		for name, value := range selectors {
			rowValue, found := row[name]
			if !found {
				return nil, false
			}
			rowString, isString := rowValue.(string)
			if !isString {
				return nil, false
			}
			if !strings.EqualFold(rowString, value) {
				selected = false
				break
			}
		}
		if selected {
			selectedRows = append(selectedRows, row)
		}
	}
	return selectedRows, true
}

func matrixSelectors(condition any) (map[string]string, bool) {
	if condition == nil {
		return nil, false
	}
	text := normalizeCondition(condition)
	if strings.HasPrefix(text, "${{") && strings.HasSuffix(text, "}}") {
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "${{"), "}}"))
	}
	if text == "" || strings.Contains(text, "${{") || strings.Contains(text, "}}") ||
		strings.Contains(text, "!") || strings.Contains(text, "||") {
		return nil, false
	}

	selectors := make(map[string]string)
	for _, rawClause := range strings.Split(text, "&&") {
		clause := trimWrappingParentheses(strings.TrimSpace(rawClause))
		if clause == "success()" {
			continue
		}
		match := matrixEqualsRE.FindStringSubmatch(clause)
		if match == nil || strings.TrimSpace(match[0]) != clause {
			return nil, false
		}
		name, value, isQuotedString := quotedMatrixSelector(match)
		if !isQuotedString {
			return nil, false
		}
		if existing, found := selectors[name]; found && existing != value {
			return nil, false
		}
		selectors[name] = value
	}
	return selectors, len(selectors) != 0
}

func quotedMatrixSelector(match []string) (string, string, bool) {
	if match[1] != "" {
		if match[2] != "" {
			return match[1], match[2], true
		}
		return "", "", false
	}
	if match[8] != "" {
		if match[5] != "" {
			return match[8], match[5], true
		}
	}
	return "", "", false
}

func trimWrappingParentheses(value string) string {
	for len(value) >= 2 && value[0] == '(' && value[len(value)-1] == ')' {
		depth := 0
		wrapsWholeValue := true
		var quote byte
		for index := 0; index < len(value); index++ {
			char := value[index]
			if quote != 0 {
				if char == quote {
					quote = 0
				}
				continue
			}
			if char == '\'' || char == '"' {
				quote = char
				continue
			}
			switch char {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 && index != len(value)-1 {
					wrapsWholeValue = false
				}
			}
			if depth < 0 {
				return value
			}
		}
		if !wrapsWholeValue || depth != 0 || quote != 0 {
			return value
		}
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

func matrixProducerContext(source job) string {
	if len(source.Strategy.Matrix) == 0 {
		return ""
	}
	encoded, err := json.Marshal(source.Strategy.Matrix)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func cachePaths(source job) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, candidate := range source.Steps {
		var candidatePaths []string
		if classifyCache(candidate) != noCache {
			candidatePaths = normalizePaths(withValue(candidate, "path"), source.Env, candidate.Env)
		}
		if path := implicitNPMCachePath(candidate, source.Env); path != "" {
			candidatePaths = append(candidatePaths, path)
		}
		for _, path := range candidatePaths {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func implicitNPMCachePath(candidate step, jobEnv map[string]any) string {
	identity := implicitNPMCacheIdentity(candidate, jobEnv)
	if identity == "" {
		return ""
	}
	return "npm-cache://" + strings.TrimPrefix(identity, "implicit-npm:")
}

func implicitNPMCacheIdentity(candidate step, jobEnv map[string]any) string {
	if actionName(candidate.Uses) != "actions/setup-node" || !setupNodeCachesNPM(candidate) {
		return ""
	}
	paths := normalizePaths(withValue(candidate, "cache-dependency-path"), jobEnv, candidate.Env)
	if len(paths) == 0 || withValue(candidate, "node-version") == "" {
		return ""
	}
	return "implicit-npm:" + withValue(candidate, "node-version") + "/" + strings.Join(paths, ",")
}

func implicitNPMWriterIdentity(candidate step, source job, repoRoot string) (string, error) {
	if !setupNodeCachesNPM(candidate) {
		return "", nil
	}
	if repoRoot != "" {
		dependencies, err := dependencyFilesForImplicitNPM(candidate, repoRoot, checkoutWorkspacePrefix(source), source.Env)
		if err != nil {
			return "", err
		}
		contentHashes := make([]string, 0, len(dependencies))
		for _, dependency := range dependencies {
			contentHashes = append(contentHashes, dependency.SHA256)
		}
		sort.Strings(contentHashes)
		configuration := struct {
			Manager string   `json:"manager"`
			Content []string `json:"content"`
		}{
			Manager: "npm",
			Content: contentHashes,
		}
		encoded, err := json.Marshal(configuration)
		if err != nil {
			return "", err
		}
		digest := sha256.Sum256(encoded)
		return "implicit-npm-writer:" + fmt.Sprintf("%x", digest[:]), nil
	}
	paths := normalizePaths(withValue(candidate, "cache-dependency-path"), source.Env, candidate.Env)
	checkoutPrefix := checkoutWorkspacePrefix(source)
	for index, dependencyPath := range paths {
		normalized := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(filepath.FromSlash(dependencyPath))), "./")
		if checkoutPrefix != "" && strings.HasPrefix(normalized, checkoutPrefix+"/") {
			normalized = strings.TrimPrefix(normalized, checkoutPrefix+"/")
		}
		paths[index] = normalized
	}
	sort.Strings(paths)
	return "implicit-npm-writer:" + strings.Join(paths, "\x00"), nil
}

func normalizePaths(value string, jobEnv, stepEnv map[string]any) []string {
	env := mergedEnv(jobEnv, stepEnv)
	seen := make(map[string]bool)
	var paths []string
	for _, raw := range strings.Split(value, "\n") {
		path := strings.TrimSpace(envRefRE.ReplaceAllStringFunc(raw, func(match string) string {
			name := envRefRE.FindStringSubmatch(match)[1]
			return env[name]
		}))
		path = strings.TrimSuffix(path, "/")
		if path != "" && !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	return paths
}

func pathsOverlap(first, second string) bool {
	return first == second || strings.HasPrefix(first, second+"/") || strings.HasPrefix(second, first+"/")
}

func mergedEnv(jobEnv, stepEnv map[string]any) map[string]string {
	values := make(map[string]string, len(jobEnv)+len(stepEnv))
	for key, value := range jobEnv {
		values[key] = fmt.Sprint(value)
	}
	for key, value := range stepEnv {
		values[key] = fmt.Sprint(value)
	}
	return values
}

func pathsPopulatedByRun(run string, knownPaths []string, jobEnv, stepEnv map[string]any) []string {
	var populated []string
	seen := make(map[string]bool)
	mark := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			populated = append(populated, path)
		}
	}
	for _, path := range knownPaths {
		if strings.Contains(run, path) {
			mark(path)
		}
	}
	env := mergedEnv(jobEnv, stepEnv)
	if goBuildRE.MatchString(run) {
		for _, path := range knownPaths {
			if path == env["GOMODCACHE"] || path == env["GOCACHE"] || path == "~/go/pkg/mod" || path == "~/.cache/go-build" {
				mark(path)
			}
		}
	}
	if goModDownloadRE.MatchString(run) {
		for _, path := range knownPaths {
			if path == env["GOMODCACHE"] || path == "~/go/pkg/mod" {
				mark(path)
			}
		}
	}
	if npmPopulateRE.MatchString(run) {
		for _, path := range knownPaths {
			if strings.Contains(strings.ToLower(path), "npm") {
				mark(path)
			}
		}
	}
	return populated
}

func resolvePrimaryKey(key string, primaryKeys map[string]string) string {
	match := primaryKeyRE.FindStringSubmatch(key)
	if match == nil {
		return key
	}
	if resolved := primaryKeys[match[1]]; resolved != "" {
		return resolved
	}
	return key
}

type cacheOperation uint8

const (
	noCache cacheOperation = iota
	restoreCache
	saveCache
	combinedCache
)

func (operation cacheOperation) restores() bool {
	return operation == restoreCache || operation == combinedCache
}
func (operation cacheOperation) saves() bool {
	return operation == saveCache || operation == combinedCache
}

func (operation cacheOperation) String() string {
	switch operation {
	case restoreCache:
		return "restore"
	case saveCache:
		return "save"
	case combinedCache:
		return "restore-save"
	default:
		return "unknown"
	}
}

func classifyCache(candidate step) cacheOperation {
	switch actionName(candidate.Uses) {
	case "actions/cache/restore":
		return restoreCache
	case "actions/cache/save":
		return saveCache
	case "actions/cache":
		return combinedCache
	default:
		return noCache
	}
}

func actionName(uses string) string {
	name, _, found := strings.Cut(uses, "@")
	if !found {
		return uses
	}
	return name
}

func withValue(candidate step, name string) string {
	if candidate.With == nil {
		return ""
	}
	value, found := candidate.With[name]
	if !found || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

type cacheKind uint8

const (
	unknownCache cacheKind = iota
	goCache
	npmCache
)

func cacheKindFor(path, key string) cacheKind {
	value := strings.ToLower(path + "\n" + key)
	switch {
	case strings.Contains(value, "go-build"), strings.Contains(value, "go/pkg/mod"), strings.Contains(value, "gomodcache"), strings.Contains(value, "gocache"):
		return goCache
	case strings.Contains(value, "npm"):
		return npmCache
	default:
		return unknownCache
	}
}

func setupNodeCacheProblems(prefix string, candidate step) []string {
	if !setupNodeCachesNPM(candidate) {
		return nil
	}
	var problems []string
	if withValue(candidate, "node-version") == "" {
		problems = append(problems, prefix+"setup-node npm cache lacks a node-version")
	}
	if withValue(candidate, "cache-dependency-path") == "" {
		problems = append(problems, prefix+"setup-node npm cache lacks cache-dependency-path")
	}
	return problems
}

func setupNodeCachesNPM(candidate step) bool {
	return strings.ToLower(withValue(candidate, "cache")) == "npm"
}

func restoreKeyProblems(prefix, key, restoreKeys string) []string {
	if key == "" || restoreKeys == "" {
		return nil
	}
	components := abiComponents(key)
	var problems []string
	for _, restoreKey := range strings.Split(restoreKeys, "\n") {
		restoreKey = strings.TrimSpace(restoreKey)
		if restoreKey == "" {
			continue
		}
		for name, value := range components {
			if !strings.Contains(restoreKey, value) {
				problems = append(problems, prefix+"restore key drops ABI component "+name)
			}
		}
	}
	return problems
}

func abiComponents(key string) map[string]string {
	known := map[string]string{
		"runner.os":      "${{ runner.os }}",
		"runner.arch":    "${{ runner.arch }}",
		"matrix.archive": "${{ matrix.archive }}",
		"matrix.runs-on": "${{ matrix.runs-on }}",
	}
	components := make(map[string]string)
	for name, value := range known {
		if strings.Contains(key, value) {
			components[name] = value
		}
	}
	for _, version := range semverRE.FindAllString(key, -1) {
		components["toolchain "+version] = version
	}
	return components
}

func jobToolchains(source job) map[cacheKind]string {
	versions := make(map[cacheKind]string)
	for _, candidate := range source.Steps {
		switch actionName(candidate.Uses) {
		case "actions/setup-go":
			versions[goCache] = withValue(candidate, "go-version")
		case "actions/setup-node":
			versions[npmCache] = withValue(candidate, "node-version")
		}
	}
	return versions
}

func cacheKeyProblems(prefix string, kind cacheKind, key string, toolchains map[cacheKind]string) []string {
	if kind == unknownCache || key == "" {
		return nil
	}
	lower := strings.ToLower(key)
	var problems []string
	switch kind {
	case goCache:
		if !strings.Contains(lower, "hashfiles(") || !strings.Contains(lower, "go.sum") {
			problems = append(problems, prefix+"Go cache key lacks a dependency hash")
		}
		if expected := toolchains[goCache]; expected != "" && !keyHasToolchainComponent(key, expected) {
			problems = append(problems, prefix+"Go cache key lacks the resolved toolchain component "+expected)
		} else if expected == "" && !goVersionRE.MatchString(lower) {
			problems = append(problems, prefix+"Go cache key lacks a toolchain component")
		}
	case npmCache:
		if !strings.Contains(lower, "hashfiles(") || !strings.Contains(lower, "package-lock.json") {
			problems = append(problems, prefix+"npm cache key lacks a dependency hash")
		}
		if expected := toolchains[npmCache]; expected != "" && !keyHasToolchainComponent(key, expected) {
			problems = append(problems, prefix+"npm cache key lacks the resolved toolchain component "+expected)
		} else if expected == "" && !nodeVersionRE.MatchString(lower) {
			problems = append(problems, prefix+"npm cache key lacks a toolchain component")
		}
	}
	return problems
}

func keyHasToolchainComponent(key, component string) bool {
	literal := expressionRE.ReplaceAllString(key, "")
	return strings.Contains(literal, "-"+component+"-") || strings.HasSuffix(literal, "-"+component) || strings.HasPrefix(literal, component+"-") || literal == component
}

type requiredUpload struct {
	workflow string
	job      string
	name     string
	paths    []string
}

var requiredUploads = []requiredUpload{
	{"browser.yml", "browser", "browser-${{ github.run_id }}-${{ github.run_attempt }}", []string{"ci-artifacts/browser/**"}},
	{"ci.yml", "gladecli", "go-test-gladecli", []string{"ci-artifacts/go-test/test-gladecli.json", "ci-artifacts/go-test/resource-gladecli.json"}},
	{"ci.yml", "node-integration", "go-test-node-integration", []string{"ci-artifacts/go-test-node-integration/test-node-integration.json", "ci-artifacts/go-test-node-integration/expected.txt", "ci-artifacts/go-test-node-integration/discovery.txt", "ci-artifacts/go-test-node-integration/validation-summary.json", "ci-artifacts/go-test-node-integration/resource-usage.json"}},
	{"ci.yml", "sema", "sema-shard-${{ matrix.shard }}", []string{"ci-artifacts/sema-${{ matrix.shard }}/"}},
	{"ci.yml", "sema-full", "sema-full", []string{"ci-artifacts/sema-full/"}},
	{"ci.yml", "sema-equivalence", "sema-equivalence-${{ github.run_id }}-${{ github.run_attempt }}", []string{"/tmp/sema-equivalence/equivalence.json"}},
	{"ci.yml", "server-and-playground", "go-test-server-and-playground", []string{"ci-artifacts/go-test/test-server-and-playground.json", "ci-artifacts/go-test/resource-server-and-playground.json"}},
	{"ci.yml", "test", "go-test-remaining-go", []string{"ci-artifacts/go-test/test-repoguard.json", "ci-artifacts/go-test/test-remaining-go.json", "ci-artifacts/go-test/resource-repoguard.json", "ci-artifacts/go-test/resource-remaining-go.json"}},
	{"ci.yml", "apextest", "apex-shard-${{ matrix.shard }}", []string{"ci-artifacts/apextest-${{ matrix.shard }}/discovery.txt", "ci-artifacts/apextest-${{ matrix.shard }}/discovery-stderr.txt", "ci-artifacts/apextest-${{ matrix.shard }}/discovery-command.txt", "ci-artifacts/apextest-${{ matrix.shard }}/plan.json", "ci-artifacts/apextest-${{ matrix.shard }}/selected-shard.json", "ci-artifacts/apextest-${{ matrix.shard }}/events.json", "ci-artifacts/apextest-${{ matrix.shard }}/validation-summary.json", "ci-artifacts/apextest-${{ matrix.shard }}/resource-usage.json"}},
	{"ci.yml", "apextest-history", "apextest-duration-history-${{ github.run_id }}-${{ github.run_attempt }}", []string{"/tmp/glade-apextest-duration-history/apextest-duration-history.json"}},
	{"race.yml", "plan", "race-plan-${{ github.run_id }}-${{ github.run_attempt }}", []string{"ci-artifacts/race-plan/plan.json"}},
	{"race.yml", "race", "race-${{ steps.artifact.outputs.slug }}", []string{"ci-artifacts/race/"}},
	{"race.yml", "race-apextest-a", "race-internal-apextest-a-${{ github.run_id }}-${{ github.run_attempt }}", []string{"ci-artifacts/race/internal-apextest"}},
	{"race.yml", "race-apextest-b", "race-internal-apextest-b-${{ github.run_id }}-${{ github.run_attempt }}", []string{"ci-artifacts/race/internal-apextest"}},
	{"race.yml", "race-apextest-aggregate", "race-internal-apextest-aggregate-${{ github.run_id }}-${{ github.run_attempt }}", []string{"ci-artifacts/race/apextest-aggregate/union-validation.json"}},
	{"release.yml", "required-ci-attestation", "required-ci-attestation", []string{"required-ci-attestation.json"}},
	{"release.yml", "shared-payload", "glade-release-shared-payload", []string{"dist/glade-shared-payload.tar.gz", "dist/glade-shared-payload.tar.gz.sha256"}},
	{"release.yml", "build", "glade-release-platform-${{ matrix.artifact }}", []string{"dist/glade_*", "dist/release-manifest-${{ matrix.artifact }}.json"}},
}

func checkRequiredUploads(workflowName string, source workflow) []string {
	var problems []string
	for _, required := range requiredUploads {
		if required.workflow != workflowName {
			continue
		}
		found := false
		enabled := false
		pathEntries := make(map[string]bool)
		for _, candidate := range source.Jobs[required.job].Steps {
			if actionName(candidate.Uses) != "actions/upload-artifact" {
				continue
			}
			if withValue(candidate, "name") == required.name {
				found = true
				if stepCannotRun(candidate) {
					continue
				}
				enabled = true
				for _, path := range normalizeArtifactPaths(withValue(candidate, "path")) {
					pathEntries[path] = true
				}
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("%s/%s: required artifact upload missing: %s", workflowName, required.job, required.name))
			continue
		}
		if !enabled {
			problems = append(problems, fmt.Sprintf("%s/%s: required artifact upload is disabled: %s", workflowName, required.job, required.name))
			continue
		}
		for _, requiredPath := range required.paths {
			if !pathEntries[requiredPath] {
				problems = append(problems, fmt.Sprintf("%s/%s: required artifact upload missing path: %s", workflowName, required.job, requiredPath))
			}
		}
	}
	return problems
}

func normalizeArtifactPaths(value string) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, raw := range strings.Split(value, "\n") {
		path := strings.TrimSpace(raw)
		if path != "" && !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	return paths
}

func stepCannotRun(candidate step) bool {
	condition := strings.TrimSpace(strings.ToLower(fmt.Sprint(candidate.If)))
	condition = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(condition, "${{"), "}}"))
	return condition == "false"
}

type cacheEvidence struct {
	Version int                   `json:"version"`
	Samples []cacheEvidenceSample `json:"samples"`
}

type cacheEvidenceSample struct {
	Cache              string  `json:"cache"`
	Sample             string  `json:"sample"`
	ObservedAt         string  `json:"observed_at"`
	TransferSeconds    float64 `json:"transfer_seconds"`
	AvoidedWorkSeconds float64 `json:"avoided_work_seconds"`
}

func checkCacheEvidence(data []byte) ([]string, error) {
	_, byCache, err := parseCacheEvidence(data)
	if err != nil {
		return nil, err
	}
	return evidenceProblems(byCache), nil
}

func checkStrictEvidence(data []byte, identities map[string]bool) ([]string, error) {
	evidence, byCache, err := parseCacheEvidence(data)
	if err != nil {
		return nil, err
	}
	if len(evidence.Samples) == 0 {
		return nil, fmt.Errorf("strict evidence is empty")
	}
	var invalid []string
	for identity, samples := range byCache {
		if !identities[identity] {
			invalid = append(invalid, "unmapped cache identity "+identity)
		}
		if len(samples) < 3 {
			invalid = append(invalid, fmt.Sprintf("incomplete evidence for %s: %d samples", identity, len(samples)))
		}
	}
	for identity := range identities {
		if len(byCache[identity]) == 0 {
			invalid = append(invalid, "incomplete evidence missing cache identity "+identity)
		}
	}
	if len(invalid) != 0 {
		sort.Strings(invalid)
		return nil, fmt.Errorf("strict evidence rejected: %s", strings.Join(invalid, "; "))
	}
	return evidenceProblems(byCache), nil
}

func parseCacheEvidence(data []byte) (cacheEvidence, map[string][]cacheEvidenceSample, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var evidence cacheEvidence
	if err := decoder.Decode(&evidence); err != nil {
		return cacheEvidence{}, nil, err
	}
	if decoder.More() {
		return cacheEvidence{}, nil, fmt.Errorf("multiple JSON values")
	}
	if evidence.Version != 1 {
		return cacheEvidence{}, nil, fmt.Errorf("unsupported version %d", evidence.Version)
	}
	byCache := make(map[string][]cacheEvidenceSample)
	observedAt := make(map[string]time.Time)
	seen := make(map[string]bool)
	for index, sample := range evidence.Samples {
		when, err := time.Parse(time.RFC3339, sample.ObservedAt)
		if sample.Cache == "" || sample.Sample == "" || err != nil || sample.TransferSeconds < 0 || sample.AvoidedWorkSeconds < 0 {
			return cacheEvidence{}, nil, fmt.Errorf("invalid sample %d", index+1)
		}
		if previous := observedAt[sample.Cache]; !previous.IsZero() && !when.After(previous) {
			return cacheEvidence{}, nil, fmt.Errorf("sample %q for cache %q is not later than the previous sample", sample.Sample, sample.Cache)
		}
		observedAt[sample.Cache] = when
		identity := sample.Cache + "\x00" + sample.Sample
		if seen[identity] {
			return cacheEvidence{}, nil, fmt.Errorf("duplicate sample %q for cache %q", sample.Sample, sample.Cache)
		}
		seen[identity] = true
		byCache[sample.Cache] = append(byCache[sample.Cache], sample)
	}
	return evidence, byCache, nil
}

func evidenceProblems(byCache map[string][]cacheEvidenceSample) []string {
	var problems []string
	for cache, samples := range byCache {
		if len(samples) < 3 {
			continue
		}
		consecutiveNegative := 0
		for _, sample := range samples {
			if sample.TransferSeconds >= sample.AvoidedWorkSeconds {
				consecutiveNegative++
			} else {
				consecutiveNegative = 0
			}
			if consecutiveNegative >= 3 {
				problems = append(problems, fmt.Sprintf("cache evidence %s is negative after %d consecutive samples", cache, consecutiveNegative))
				break
			}
		}
	}
	sort.Strings(problems)
	return problems
}
