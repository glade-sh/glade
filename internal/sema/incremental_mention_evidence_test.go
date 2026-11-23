package sema

import (
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// TestIncrementalMentionNarrowingEvidence checks SkipMentionRechecks. By
// default it runs the bundled projects in incrementalMentionEvidenceProjects;
// GLADE_SEMA_MENTION_EVIDENCE_ROOT names another project to read instead.
// Every project class is the edited class in turn, with up to three edits of random kinds
// applied through a source overlay (the project is only read). From one
// baseline snapshot, the full rule rechecks the edited class, its subtypes
// and every class that names it. A narrowed run reuses the cached diagnostics
// of the classes that only name it, so it equals the full rule exactly when
// each of those classes' fresh diagnostics equal its cached ones; any
// difference is reported with the class index and edit.
//
// GLADE_SEMA_MENTION_EVIDENCE_SHARD=i/n checks every n-th file from i;
// GLADE_SEMA_MENTION_EVIDENCE_LIMIT caps the files checked.
func TestIncrementalMentionNarrowingEvidence(t *testing.T) {
	if root := os.Getenv("GLADE_SEMA_MENTION_EVIDENCE_ROOT"); root != "" {
		runIncrementalMentionEvidence(t, root, 3)
		return
	}
	kindEdits, kindMentions := map[string]int{}, map[string]int{}
	for _, rel := range incrementalMentionEvidenceProjects {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			root := t.TempDir()
			incrementalCopyTree(t, filepath.Join("..", "..", rel), root)
			// The small evidence project applies every edit kind to every
			// file, so no kind depends on a random pick.
			perFile := 3
			if strings.HasSuffix(rel, "incremental-edit-kinds") {
				perFile = len(incrementalEdits)
			}
			edits, mentions := runIncrementalMentionEvidence(t, root, perFile)
			for name, n := range edits {
				kindEdits[name] += n
			}
			for name, n := range mentions {
				kindMentions[name] += n
			}
		})
	}
	for _, name := range incrementalMentionRequiredKinds {
		t.Logf("REQUIRED KIND %s edits=%d comparisons=%d", name, kindEdits[name], kindMentions[name])
		if kindEdits[name] == 0 || kindMentions[name] == 0 {
			t.Errorf("edit kind %s ended with %d edits and %d mention comparisons across the bundled projects", name, kindEdits[name], kindMentions[name])
		}
	}
}

// incrementalMentionEvidenceProjects are bundled projects whose classes name
// each other, so mention-only classes exist for most edits.
var incrementalMentionEvidenceProjects = []string{
	"testdata/local-tests/enterprise-composed",
	"testdata/local-tests/org-like-runner",
	"testdata/local-tests/platform-apis",
	// A small synthetic project: a static block, a property with accessor
	// bodies, an inner type method and a multi-field query, all named by
	// other classes.
	"internal/sema/testdata/incremental-edit-kinds",
}

// incrementalMentionRequiredKinds are edit kinds that must produce at least
// one edit and one mention-only comparison across the bundled projects.
var incrementalMentionRequiredKinds = []string{"static-block-body", "accessor-body", "inner-type-body", "soql-field-list"}

func runIncrementalMentionEvidence(t *testing.T, root string, editsPerFile int) (map[string]int, map[string]int) {
	t.Helper()
	shard, shards := 0, 1
	if spec := os.Getenv("GLADE_SEMA_MENTION_EVIDENCE_SHARD"); spec != "" {
		left, right, _ := strings.Cut(spec, "/")
		shard, _ = strconv.Atoi(left)
		shards, _ = strconv.Atoi(right)
		if shards < 1 || shard < 0 || shard >= shards {
			t.Fatalf("bad shard %q", spec)
		}
	}
	limit, _ := strconv.Atoi(os.Getenv("GLADE_SEMA_MENTION_EVIDENCE_LIMIT"))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	state := NewIncrementalState()
	start := time.Now()
	_, run := state.analyzeLocked(index, incrementalOptions(&artifacts), nil)
	if run.snapshot == nil {
		t.Fatalf("baseline not cached: %+v", run.stats)
	}
	base := run.snapshot
	seen := map[string]bool{}
	var files []string
	for _, typ := range index.Types {
		if typ.Dependency || typ.Artifact || !typ.HasSourceSnapshot() || !strings.HasSuffix(typ.File, ".cls") || seen[typ.File] {
			continue
		}
		seen[typ.File] = true
		files = append(files, typ.File)
	}
	sort.Strings(files)
	rng := rand.New(rand.NewSource(42))
	rng.Shuffle(len(files), func(i, j int) { files[i], files[j] = files[j], files[i] })
	// Every edit kind that can leave the index unchanged; kinds that change it
	// are counted as not body-only.
	var edits []incrementalEdit
	for _, e := range incrementalEdits {
		switch e.name {
		case "comment-at-end", "comment-at-top", "blank-lines-top", "add-method", "add-field", "break-syntax":
			continue
		}
		edits = append(edits, e)
	}
	var checked, compared, notBodyOnly, noBody, fellBack, mentionTypes, differ int
	kindEdits, kindMentions := map[string]int{}, map[string]int{}
	for fi, file := range files {
		if fi%shards != shard {
			continue
		}
		if limit > 0 && checked >= limit {
			break
		}
		checked++
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		// One generator per file keeps each file's edits independent of sharding.
		fileRNG := rand.New(rand.NewSource(int64(fi) + 1))
		applied := 0
		for _, pick := range fileRNG.Perm(len(edits)) {
			if applied == editsPerFile {
				break
			}
			name := edits[pick].name
			text, ok := edits[pick].apply(string(raw), fileRNG)
			if !ok {
				continue
			}
			applied++
			overlay := typesys.NewSourceOverlay(map[string]string{file: text})
			next, exact := typesys.TryUpdateApexFileWithOverlay(index, file, p, overlay)
			if !exact {
				notBodyOnly++
				continue
			}
			nextArtifacts, err := typesys.RefreshBuildArtifactsWithOverlay(next, &artifacts, overlay)
			if err != nil {
				notBodyOnly++
				continue
			}
			_, full := state.analyzeLocked(next, incrementalOptions(&nextArtifacts), base)
			if !full.reuseOK {
				notBodyOnly++
				continue
			}
			if full.fallback || full.snapshot == nil {
				fellBack++
				continue
			}
			compared++
			kindEdits[name]++
			narrowed, _ := subtypeClosure(next, full.changed)
			for typeIndex := range next.Types {
				if full.recheck == nil || !full.recheck[typeIndex] || narrowed[typeIndex] {
					continue
				}
				mentionTypes++
				kindMentions[name]++
				if !reflect.DeepEqual(full.snapshot.typeDiags[typeIndex], base.typeDiags[typeIndex]) {
					differ++
					t.Errorf("file #%d edit %s: class #%d names the edited class and its diagnostics changed: %d -> %d", fi, name, typeIndex, len(base.typeDiags[typeIndex]), len(full.snapshot.typeDiags[typeIndex]))
				}
			}
		}
		if applied == 0 {
			noBody++
		}
		if checked%25 == 0 {
			t.Logf("progress files=%d edits=%d mentionTypes=%d differ=%d elapsed=%s", checked, compared, mentionTypes, differ, time.Since(start).Round(time.Second))
		}
	}
	kinds := make([]string, 0, len(kindEdits))
	for name := range kindEdits {
		kinds = append(kinds, name)
	}
	sort.Strings(kinds)
	for _, name := range kinds {
		t.Logf("KIND %s bodyOnlyEdits=%d mentionOnlyTypesCompared=%d", name, kindEdits[name], kindMentions[name])
	}
	t.Logf("SUMMARY shard=%d/%d files=%d of %d bodyOnlyEdits=%d mentionOnlyTypesCompared=%d differ=%d notBodyOnly=%d noBody=%d fallback=%d elapsed=%s",
		shard, shards, checked, len(files), compared, mentionTypes, differ, notBodyOnly, noBody, fellBack, time.Since(start).Round(time.Second))
	return kindEdits, kindMentions
}
