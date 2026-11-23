package sema

import (
	"math/rand"
	"os"
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

// TestIncrementalMentionNarrowingEvidence checks SkipMentionRechecks on a
// project named by GLADE_SEMA_MENTION_EVIDENCE_ROOT; it is skipped otherwise.
// Every project class is the edited class in turn, with two random body edits
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
	root := os.Getenv("GLADE_SEMA_MENTION_EVIDENCE_ROOT")
	if root == "" {
		t.Skip("GLADE_SEMA_MENTION_EVIDENCE_ROOT is not set")
	}
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
	edits := []string{"local-in-body", "undefined-call-in-body", "unknown-field-in-body", "unknown-type-in-body", "duplicate-local-in-body"}
	editFn := map[string]incrementalEdit{}
	for _, e := range incrementalEdits {
		editFn[e.name] = e
	}
	var checked, compared, notBodyOnly, noBody, fellBack, mentionTypes, differ int
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
		for k := 0; k < 2; k++ {
			name := edits[fileRNG.Intn(len(edits))]
			text, ok := editFn[name].apply(string(raw), fileRNG)
			if !ok {
				noBody++
				continue
			}
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
			narrowed := narrowedRecheckSet(next, full.changed)
			for typeIndex := range next.Types {
				if full.recheck == nil || !full.recheck[typeIndex] || narrowed[typeIndex] {
					continue
				}
				mentionTypes++
				if !reflect.DeepEqual(full.snapshot.typeDiags[typeIndex], base.typeDiags[typeIndex]) {
					differ++
					t.Errorf("file #%d edit %s: class #%d names the edited class and its diagnostics changed: %d -> %d", fi, name, typeIndex, len(base.typeDiags[typeIndex]), len(full.snapshot.typeDiags[typeIndex]))
				}
			}
		}
		if checked%25 == 0 {
			t.Logf("progress files=%d edits=%d mentionTypes=%d differ=%d elapsed=%s", checked, compared, mentionTypes, differ, time.Since(start).Round(time.Second))
		}
	}
	t.Logf("SUMMARY shard=%d/%d files=%d of %d bodyOnlyEdits=%d mentionOnlyTypesCompared=%d differ=%d notBodyOnly=%d noBody=%d fallback=%d elapsed=%s",
		shard, shards, checked, len(files), compared, mentionTypes, differ, notBodyOnly, noBody, fellBack, time.Since(start).Round(time.Second))
}

// narrowedRecheckSet is the SkipMentionRechecks set: the changed types and
// their subtypes, transitively.
func narrowedRecheckSet(index typesys.Index, changed []bool) []bool {
	recheck := append([]bool(nil), changed...)
	names := map[string]bool{}
	for i, typ := range index.Types {
		if changed[i] {
			names[strings.ToLower(topLevelTypeName(typ.Name))] = true
		}
	}
	for grown := true; grown; {
		grown = false
		for i, typ := range index.Types {
			if !recheck[i] && extendsAny(typ, names) {
				recheck[i] = true
				names[strings.ToLower(topLevelTypeName(typ.Name))] = true
				grown = true
			}
		}
	}
	return recheck
}
