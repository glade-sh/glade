package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
)

// Fixture mutations check that each block can turn the project gate red. These
// are test-sensitivity checks, not native observations of rejected Apex.
// The deployable control and revised opaque/System receiver shapes are backed by
// testdata/corpus_shape/native/org.tsv:6,11 and its ORG-RUN.txt capture note;
// the invalid original shapes were rejected in org.tsv:2-5,7-10.
func TestCorpusShapeMutationCheck(t *testing.T) {
	control := loadTestIndex(t, copyCorpusShapeProject(t, "67.0"))
	if len(control.Diagnostics) != 0 {
		t.Fatal(control.Diagnostics)
	}
	if result := sema.Analyze(control); len(result.Diagnostics) != 0 {
		t.Fatal(result.Diagnostics)
	}
	if run := Run(control, Options{NoDiskCache: true, Parallelism: 1}); run.Summary().Passed != 14 || run.Summary().Total != 14 {
		t.Fatalf("unmutated control failed: %+v; %s", run.Summary(), firstRunProblem(run))
	}
	expectedDiagnostic := map[string]string{
		"opaqueProvider": "AbsentEnvelope", "standardChildren": "AbsentChildren",
		"customChildren": "AbsentChildren__r", "domainFactory": "returns String",
		"unitOfWork": "absentRollback", "enum": "absentName", "lexicalSystem": "EventBus.absent",
		"staticInstance": "Absent", "namespaceReceiver": "AbsentOuter", "selector": "List<Contact>",
		"dispatcher": "absentBeforeInsert",
	}
	assertionMutation := map[string]bool{"changeEventIndustry": true, "topScheduler": true, "innerScheduler": true, "batch": true, "queueable": true}

	const classes = "force-app/main/default/classes/"
	for _, m := range []struct{ name, file, before, after, method string }{
		{"opaqueProvider", classes + "ShapeOpaque.cls", "System.Type provider", "AbsentEnvelope provider", ""},
		{"standardChildren", classes + "ShapeRelationships.cls", "FROM Contacts", "FROM AbsentChildren", ""},
		{"customChildren", classes + "ShapeRelationships.cls", "FROM Leaves__r", "FROM AbsentChildren__r", ""},
		{"domainFactory", classes + "ShapeDomain.cls", "return new ShapeDomain(records);", "return 'not a domain';", ""},
		{"unitOfWork", classes + "ShapeWork.cls", "Database.rollback(boundary)", "Database.absentRollback(boundary)", ""},
		{"enum", classes + "ShapeLevels.cls", "Level.NONE.name()", "Level.NONE.absentName()", "enumReceivers"},
		{"lexicalSystem", classes + "ShapeShadow.cls", "EventBus.publish.name()", "EventBus.absent.name()", "lexicalSystemNames"},
		{"staticInstance", classes + "ShapeOuter.cls", "ShapeOuter.Instance.Text", "ShapeOuter.Instance.Absent", "staticInstanceReceivers"},
		{"namespaceReceiver", classes + "ShapeOuter.cls", "System.String.valueOf", "System.AbsentOuter.valueOf", "staticInstanceReceivers"},
		{"selector", classes + "ShapeSelector.cls", "FROM Account WHERE Id IN", "FROM Contact WHERE Id IN", "selectorDomainWorkDispatcher"},
		{"dispatcher", classes + "ShapeDispatcher.cls", "domain.beforeInsert()", "domain.absentBeforeInsert()", "selectorDomainWorkDispatcher"},
		{"changeEventIndustry", classes + "ShapeProjectTest.cls", "containsKey('industry')", "containsKey('absentIndustry')", "changeEventIndustrySurface"},
		{"topScheduler", classes + "ShapeSchedule.cls", "'shape-top'", "'broken-top'", "topLevelSchedulableAtStopTest"},
		{"innerScheduler", classes + "ShapeProjectTest.cls", "new ShapePlanner.Tick()", "new ShapeSchedule()", "innerSchedulableAtStopTest"},
		{"batch", classes + "ShapeBatch.cls", "'shape-batch'", "'broken-batch'", "batchToQueueable"},
		{"queueable", classes + "ShapeQueue.cls", "'shape-queue'", "'broken-queue'", "queueableToQueueable"},
	} {
		t.Run(m.name, func(t *testing.T) {
			root := copyCorpusShapeProject(t, "67.0")
			path := filepath.Join(root, m.file)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), m.before) {
				t.Fatal("mutation target missing")
			}
			mutated := strings.ReplaceAll(string(body), m.before, m.after)
			if m.name == "innerScheduler" {
				mutated = strings.ReplaceAll(mutated, "WHERE Name = 'shape-inner'", "WHERE Name = 'shape-top'")
			}
			writeFile(t, path, mutated)
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if len(analysis.Diagnostics) > 0 {
				needle := expectedDiagnostic[m.name]
				if !assertionMutation[m.name] && needle != "" {
					for _, diagnostic := range analysis.Diagnostics {
						if strings.Contains(diagnostic.Message, needle) {
							t.Logf("DETECTED by check: %s", diagnostic.Message)
							return
						}
					}
				}
				t.Fatalf("unrelated diagnostics: %v", analysis.Diagnostics)
			}
			if m.method == "" {
				t.Fatal("MISSED compile-only mutation")
			}
			run := Run(index, Options{SelectedClasses: []string{"ShapeProjectTest"}, SelectedMethod: m.method, NoDiskCache: true, Parallelism: 1})
			s := run.Summary()
			if s.Total != 1 || s.Passed != 0 || s.Skipped != 0 || s.CompileErrors != 0 || s.ExcludedClasses != 0 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("MISSED or invalid result: %+v; %s", s, firstRunProblem(run))
			}
			item := run.Suites[0].Cases[0]
			if assertionMutation[m.name] {
				if item.Status != testreport.StatusFail || item.Reason != testreport.ReasonAssertion || s.Failed != 1 || s.Errors != 0 || s.Unsupported != 0 {
					t.Fatalf("expected assertion failure: %+v; %+v", s, item)
				}
			} else {
				// A missing member can reach the VM's unsupported-call boundary;
				// a missing instance field can instead throw an Apex exception.
				// Match the mutated member as well as the terminal reason so an
				// unrelated unsupported feature or infrastructure error cannot count.
				runtimeRejection := (item.Status == testreport.StatusFail || item.Status == testreport.StatusRuntimeError) && item.Reason == testreport.ReasonRuntimeError
				unsupportedCall := item.Status == testreport.StatusUnsupported && item.Reason == testreport.ReasonUnsupportedFeature
				needle := expectedDiagnostic[m.name]
				if (!runtimeRejection && !unsupportedCall) || item.Problem == nil || needle == "" || !strings.Contains(item.Problem.Message, needle) {
					t.Fatalf("expected targeted member rejection: %+v; %+v", s, item)
				}
			}
			t.Logf("DETECTED by runner: %s", firstRunProblem(run))
		})
	}
	t.Run("anonymousPostReturn", func(t *testing.T) {
		// throw_successor/org.tsv:2,18 accepts its captured anonymous
		// direct_throw form. Do not substitute throw for this fixture's return:
		// mutate only the successor's expression type, a compile-only assertion.
		for _, api := range []string{"62.0", "67.0"} {
			t.Run("API"+api, func(t *testing.T) {
				root := copyCorpusShapeProject(t, api)
				index := loadTestIndex(t, root)
				source, err := os.ReadFile(filepath.Join(root, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				const before = "ShapeReturn.compileRoute()"
				if !strings.Contains(string(source), before) {
					t.Fatal("mutation target missing")
				}
				if result := sema.AnalyzeAnonymous(index, string(source), api); len(result.Diagnostics) != 0 {
					t.Fatalf("unmutated anonymous control failed: %v", result.Diagnostics)
				}
				mutated := strings.ReplaceAll(string(source), before, "System.debug((AbsentPostReturn)null)")
				result := sema.AnalyzeAnonymous(index, mutated, api)
				for _, diagnostic := range result.Diagnostics {
					if diagnostic.Code == "GLADESEMA006" && strings.Contains(diagnostic.Message, "AbsentPostReturn") {
						t.Logf("DETECTED by anonymous check: %s", diagnostic.Message)
						return
					}
				}
				t.Fatalf("MISSED post-return expression-type mutation: %v", result.Diagnostics)
			})
		}
	})
}
