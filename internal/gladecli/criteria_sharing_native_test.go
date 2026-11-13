package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"

	"github.com/glade-sh/glade/internal/dap"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const criteriaSharingFixtures = "../apextest/testdata/conformance"

// NATIVE a criteria-based sharing rule grants Read to its target group when a
// transaction commits, not inside it. Each step below is its own glade exec
// transaction against one persistent database, in the order of the native rows:
// the inserting transaction sees no access row grant and zero Rule shares, the
// next one sees the grant and keeps seeing it, an update into the rule grants at
// its commit, and an update out of the rule revokes at its commit with the
// updating transaction still reading the old state. A record that never
// matches never gets a grant.
// testdata/conformance/criteria_sharing/grant_observations.tsv, every row but
// named-control (that one is internal/apextest/criteria_sharing_native_test.go).
func TestCriteriaSharingCommitGrantNative(t *testing.T) {
	rows := readCriteriaTSV(t, filepath.Join(criteriaSharingFixtures, "criteria_sharing", "grant_observations.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root, db, reader := criteriaSharingProject(t, api)
			checked := 0
			for _, row := range rows {
				if row["api"] != api || row["case"] == "named-control" {
					continue
				}
				script := criteriaCheckpointScript(t, row["case"], row["step"], reader)
				got := runCriteriaExec(t, root, db, script)
				if got != row["observation"] {
					t.Errorf("%s %s: Glade=%q native=%q", row["case"], row["step"], got, row["observation"])
				}
				checked++
			}
			if checked != 11 {
				t.Fatalf("checked %d native checkpoints for %s, want 11", checked, api)
			}
		})
	}
}

// NATIVE the anonymous Case-phase rows (case_phase/observations.tsv lines 2-3,
// both APIs): from anonymous Apex, another internal user's UserRecordAccess row
// for a Private Case exists and is false inside the inserting transaction and
// right after save, the Case has zero Rule shares in the trigger, and the
// escalation fields hold. The route has no test context.
func TestCriteriaSharingCasePhaseAnonymousNative(t *testing.T) {
	rows := readCriteriaTSV(t, filepath.Join(criteriaSharingFixtures, "case_phase", "observations.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			want := ""
			for _, row := range rows {
				if row["route"] == "anonymous" && row["api"] == api {
					want = strings.TrimPrefix(row["observation"], "OBS|")
				}
			}
			if want == "" {
				t.Fatalf("no anonymous native row for %s", api)
			}
			root, db, reader := criteriaSharingProject(t, api)
			got := runCriteriaExec(t, root, db, `
CasePhaseCapture.readerId = '`+reader+`';
CasePhaseCapture.submitterId = UserInfo.getUserId();
CasePhaseCapture.escalationQueueId = [SELECT Id FROM Group WHERE DeveloperName = 'CaseEscalationTarget' AND Type = 'Queue' LIMIT 1].Id;
CasePhaseCapture.afterInsertAccess = 'unset';
CasePhaseCapture.afterInsertShareRows = 'unset';
CasePhaseCapture.escalationDuringState = 'unset';
Case shared = new Case(Subject = 'NATIVE_CASE_SHARE_PROBE', Status = 'New', Origin = 'Web');
insert shared;
Case control = new Case(Subject = 'NATIVE_CASE_NO_SHARE', Status = 'New', Origin = 'Web');
insert control;
Case escalation = new Case(Subject = 'NATIVE_CASE_ESCALATION_PROBE', Status = 'New', Origin = 'Web');
insert escalation;
List<UserRecordAccess> matchAccess = [SELECT RecordId, HasReadAccess FROM UserRecordAccess WHERE UserId = :CasePhaseCapture.readerId AND RecordId = :shared.Id];
List<UserRecordAccess> controlAccess = [SELECT RecordId, HasReadAccess FROM UserRecordAccess WHERE UserId = :CasePhaseCapture.readerId AND RecordId = :control.Id];
List<Case> escalationRows = [SELECT OwnerId, IsEscalated FROM Case WHERE Id = :escalation.Id];
System.debug('OBS|case-share|during=' + CasePhaseCapture.afterInsertAccess + ':' + CasePhaseCapture.afterInsertShareRows
 + '|after=' + (matchAccess.isEmpty() ? 'no-row' : String.valueOf(matchAccess[0].HasReadAccess)) + ',' + (controlAccess.isEmpty() ? 'no-row' : String.valueOf(controlAccess[0].HasReadAccess))
 + '|escalation=' + CasePhaseCapture.escalationDuringState + '>' + String.valueOf(escalationRows[0].IsEscalated) + ':' + CasePhaseCapture.ownerKind(escalationRows[0].OwnerId));`)
			if got != want {
				t.Errorf("Glade=%q native=%q", got, want)
			}
		})
	}
}

// The commit hook is the one place the grant appears: a dry run commits
// nothing and recalculates nothing.
func TestCriteriaSharingDryRunDoesNotGrant(t *testing.T) {
	root, db, reader := criteriaSharingProject(t, "67.0")
	runCriteriaExec(t, root, db, criteriaCheckpointScript(t, "commit-0", "tx1-same-tx", reader))
	before := runCriteriaExec(t, root, db, criteriaCheckpointScript(t, "commit-0", "tx2-next", reader))
	if !strings.Contains(before, "M:subj=NATIVE_CASE_SHARE_PROBE:ura=true:rule=1") {
		t.Fatalf("grant missing after commit: %q", before)
	}
	runCriteriaExec(t, root, db, criteriaCheckpointScript(t, "update-out", "same-tx", reader), "--dry-run")
	if after := runCriteriaExec(t, root, db, criteriaCheckpointScript(t, "update-out", "next-tx", reader)); !strings.Contains(after, "M:subj=NATIVE_CASE_SHARE_PROBE:ura=true:rule=1") {
		t.Fatalf("dry run changed committed state: %q", after)
	}
}

func readCriteriaTSV(t *testing.T, path string) []map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	header := strings.Split(lines[0], "\t")
	var rows []map[string]string
	for _, line := range lines[1:] {
		cols := strings.Split(line, "\t")
		row := map[string]string{}
		for i, name := range header {
			if i < len(cols) {
				row[name] = cols[i]
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// criteriaSharingProject copies the case_phase project (Private Case, one
// criteria rule to a public group), seeds one Standard User reader as a direct
// group member, and returns the project root, the database path and the reader.
func criteriaSharingProject(t *testing.T, api string) (root, db, reader string) {
	t.Helper()
	root = t.TempDir()
	source := filepath.Join(criteriaSharingFixtures, "case_phase")
	err := filepath.WalkDir(filepath.Join(source, "project"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.Join(source, "project"), path)
		if err != nil {
			return err
		}
		writeTestFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "67.0", api))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	scratch, err := os.ReadFile(filepath.Join(source, "project-scratch-def.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(scratch))
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	db = filepath.Join(root, "state.sqlite")
	profile := criteriaQueryID(t, root, db, "SELECT Id FROM Profile WHERE Name = 'Standard User'")
	group := criteriaQueryID(t, root, db, "SELECT Id FROM Group WHERE DeveloperName = 'CasePhaseReaders' AND Type = 'Regular'")
	fixture := filepath.Join(root, "reader-fixture.json")
	writeTestFile(t, fixture, `{"version":"glade.storage.v1","objects":[`+
		`{"name":"User","records":[{"alias":"reader","fields":{"Username":{"kind":"string","string":"criteria.reader@example.invalid"},"LastName":{"kind":"string","string":"Reader"},"Alias":{"kind":"string","string":"crit"},"Email":{"kind":"string","string":"reader@example.invalid"},"ProfileId":{"kind":"id","id":"`+profile+`"}}}]},`+
		`{"name":"GroupMember","records":[{"fields":{"GroupId":{"kind":"id","id":"`+group+`"}},"fieldRefs":{"UserOrGroupId":"reader"}}]}]}`)
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"db", "seed", "--project", root, "--db", db, fixture}, &stdout, &stderr); code != 0 {
		t.Fatalf("seed failed: %s%s", stdout.String(), stderr.String())
	}
	return root, db, criteriaQueryID(t, root, db, "SELECT Id FROM User WHERE Alias = 'crit'")
}

func criteriaQueryID(t *testing.T, root, db, soql string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"db", "query", "--project", root, "--db", db, "--json", soql}, &stdout, &stderr); code != 0 {
		t.Fatalf("query %q failed: %s%s", soql, stdout.String(), stderr.String())
	}
	var envelope struct {
		Data struct {
			Records []map[string]any `json:"records"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || len(envelope.Data.Records) != 1 {
		t.Fatalf("query %q: %v %s", soql, err, stdout.String())
	}
	id, _ := envelope.Data.Records[0]["Id"].(string)
	if id == "" {
		t.Fatalf("query %q returned no Id", soql)
	}
	return id
}

// runCriteriaExec runs one anonymous transaction and returns its OBS| line.
func runCriteriaExec(t *testing.T, root, db, script string, extra ...string) string {
	t.Helper()
	logPath := filepath.Join(root, "exec.log")
	_ = os.Remove(logPath)
	args := append([]string{"--project", root, "--db", db, "--log-out", logPath}, extra...)
	args = append(args, script)
	if err := runExec(context.Background(), args, new(bytes.Buffer)); err != nil {
		t.Fatalf("exec failed: %v\n%s", err, script)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(log), "\n") {
		if _, after, ok := strings.Cut(line, "|DEBUG|OBS|"); ok {
			return after
		}
	}
	t.Fatalf("no OBS line in log:\n%s", log)
	return ""
}

// criteriaCheckpointScript builds the transaction for one native step. Probe
// records are found by SuppliedName: N is the record outside the rule, M the one
// inside it. The observation text is the native row's format.
func criteriaCheckpointScript(t *testing.T, caseName, step, reader string) string {
	t.Helper()
	label := caseName + "." + step
	switch {
	case caseName == "update-into" && step == "plus-30s", caseName == "update-out" && step == "plus-30s":
		label = caseName + ".30s"
	case caseName == "cleanup":
		label = "cleanup"
	}
	var action string
	switch caseName + "/" + step {
	case "commit-0/tx1-same-tx":
		action = `insert new List<Case>{
 new Case(Subject = 'R450G_NOMATCH', SuppliedName = 'N', Status = 'New', Origin = 'Web'),
 new Case(Subject = 'NATIVE_CASE_SHARE_PROBE', SuppliedName = 'M', Status = 'New', Origin = 'Web')};`
	case "update-into/same-tx":
		action = `Case n = [SELECT Id FROM Case WHERE SuppliedName = 'N']; n.Subject = 'NATIVE_CASE_SHARE_PROBE'; update n;`
	case "update-out/same-tx":
		action = `Case m = [SELECT Id FROM Case WHERE SuppliedName = 'M']; m.Subject = 'R450G_AWAY'; update m;`
	case "cleanup/delete":
		return `delete [SELECT Id FROM Case WHERE SuppliedName IN ('N', 'M')];
System.debug('OBS|cleanup|remaining=' + [SELECT COUNT() FROM Case WHERE SuppliedName IN ('N', 'M')]);`
	}
	return `Id reader = '` + reader + `';
` + action + `
String out = '` + label + `';
for (String tag : new List<String>{'N', 'M'}) {
 Case c = [SELECT Id, Subject FROM Case WHERE SuppliedName = :tag];
 List<UserRecordAccess> access = [SELECT RecordId, HasReadAccess FROM UserRecordAccess WHERE UserId = :reader AND RecordId = :c.Id];
 out += '|' + tag + ':subj=' + c.Subject + ':ura=' + (access.isEmpty() ? 'no-row' : String.valueOf(access[0].HasReadAccess))
  + ':rule=' + [SELECT COUNT() FROM CaseShare WHERE CaseId = :c.Id AND RowCause = 'Rule'];
}
System.debug('OBS|' + out);`
}

func TestCriteriaSharingDebugCommitGrantAndRevoke(t *testing.T) {
	root, db, _ := criteriaSharingProject(t, "67.0")
	debug := func(source string) {
		t.Helper()
		inR, inW := io.Pipe()
		outR, outW := io.Pipe()
		done := make(chan error, 1)
		go func() {
			defer outW.Close()
			done <- runDAP(context.Background(), []string{"--project", root, "--db", db}, inR, outW)
		}()
		messages := make(chan map[string]any, 16)
		go readDAPMessages(t, outR, messages)
		writeDAPRequest(t, inW, dap.CommandInitialize, 1, nil)
		writeDAPRequest(t, inW, dap.CommandLaunch, 2, map[string]any{"source": source})
		writeDAPRequest(t, inW, dap.CommandConfigurationDone, 3, nil)
		stderr := waitForDAPTerminatedAndStderr(t, messages)
		if stderr != "" {
			t.Fatalf("debug error: %s", stderr)
		}
		_ = inW.Close()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	observe := func() string {
		return runCriteriaExec(t, root, db, `System.debug('OBS|' + [SELECT COUNT() FROM CaseShare WHERE RowCause = 'Rule']);`)
	}
	debug(`insert new Case(Subject='NATIVE_CASE_SHARE_PROBE', Status='New', Origin='Web');`)
	if got := observe(); got != "1" {
		t.Fatalf("grant count = %s", got)
	}
	debug(`Case c = [SELECT Id FROM Case WHERE Subject='NATIVE_CASE_SHARE_PROBE' LIMIT 1]; c.Subject='OTHER'; update c;`)
	if got := observe(); got != "0" {
		t.Fatalf("revoke count = %s", got)
	}
}
