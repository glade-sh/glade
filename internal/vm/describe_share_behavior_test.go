package vm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These rows pin what describe values do today in a VM that builds its own
// describes, including glade's handling of writes to describe maps and to
// field and type tokens. Every row must give the same output in an unprimed
// VM and in clones of a primed runtime template, whether the clone builds a
// describe first or reuses one another clone built, and a clone's writes must
// never reach its siblings. The expected values were recorded from the
// implementation before describe sharing, with heap samples updated for
// reference identity accounting backed by native retained-root controls.
// These are local behavior pins,
// including accepted token writes; they do not claim native parity.
type describeShareRow struct {
	name   string
	source string
	debug  string
	err    string
	// namespaced rows run against describeShareNamespacedOrg.
	namespaced bool
}

func describeShareRows() []describeShareRow {
	return []describeShareRow{
		{
			name: "global describe map writes persist in the VM",
			source: `Schema.getGlobalDescribe().put('zzshare', Account.SObjectType);
System.debug('P1a:' + Schema.getGlobalDescribe().containsKey('zzshare') + ':' + Schema.getGlobalDescribe().size());
Schema.getGlobalDescribe().remove('account');
System.debug('P1b:' + Schema.getGlobalDescribe().containsKey('account') + ':' + Schema.getGlobalDescribe().get('account') + ':' + Schema.getGlobalDescribe().size());
Schema.getGlobalDescribe().clear();
System.debug('P1c:' + Schema.getGlobalDescribe().size() + ':' + Schema.getGlobalDescribe().keySet().size() + ':' + Schema.getGlobalDescribe().get('widget__c'));`,
			debug: "P1a:true:21 | P1b:true:Account:20 | P1c:0:0:Widget__c",
		},
		{
			name: "typed global describe variable is a private copy",
			source: `Map<String, Schema.SObjectType> gd = Schema.getGlobalDescribe();
Integer n = gd.size();
gd.put('zzshare', Account.SObjectType);
gd.remove('account');
System.debug('P2a:' + gd.containsKey('zzshare') + ':' + (gd.size() - n) + ':' + gd.get('account'));
System.debug('P2b:' + Schema.getGlobalDescribe().containsKey('zzshare') + ':' + (Schema.getGlobalDescribe().size() - n));`,
			debug: "P2a:true:0:Account | P2b:false:0",
		},
		{
			name: "fields.getMap put is read-only",
			source: `Map<String, Schema.SObjectField> fm = Schema.SObjectType.Account.fields.getMap();
fm.put('zzshare', Account.Name);`,
			err: "System.FinalException: Collection is read-only",
		},
		{
			name: "fields.getMap remove is read-only",
			source: `Map<String, Schema.SObjectField> fm = Account.SObjectType.getDescribe().fields.getMap();
fm.remove('name');`,
			err: "System.FinalException: Collection is read-only",
		},
		{
			name:   "fields.getMap clear is read-only",
			source: `Account.SObjectType.getDescribe().fields.getMap().clear();`,
			err:    "System.FinalException: Collection is read-only",
		},
		{
			name: "fields wrapper put is read-only",
			source: `Schema.DescribeSObjectResult d = Account.SObjectType.getDescribe();
d.fields.put('zzshare', Account.Name);`,
			err: "System.FinalException: Collection is read-only",
		},
		{
			name:   "fields wrapper remove is read-only",
			source: `Schema.SObjectType.Account.fields.remove('name');`,
			err:    "System.FinalException: Collection is read-only",
		},
		{
			name: "fields wrapper reads then clear is read-only",
			source: `System.debug('P6a:' + Schema.SObjectType.Account.fields.size() + ':' + Schema.SObjectType.Account.fields.getMap().size() + ':' + Schema.SObjectType.Account.fields.get('name') + ':' + Schema.SObjectType.Account.fields.containsKey('NAME'));
Schema.SObjectType.Account.fields.clear();`,
			debug: "P6a:62:62:Name:true",
			err:   "System.FinalException: Collection is read-only",
		},
		{
			name: "describeSObjects fields wrapper put is read-only",
			source: `List<Schema.DescribeSObjectResult> ds = Schema.describeSObjects(new List<String>{'Account'});
ds[0].fields.put('zzshare', Account.Name);`,
			err: "System.FinalException: Collection is read-only",
		},
		{
			name: "field token label write",
			source: `Schema.SObjectField t = Schema.SObjectType.Account.fields.getMap().get('name');
t.label = 'zz';
System.debug('T1:' + t.label + ':' + Schema.SObjectType.Account.fields.getMap().get('Name').label + ':' + Account.SObjectType.getDescribe().fields.getMap().get('account.name').label + ':' + Schema.SObjectType.Account.fields.Name.label);
System.debug('T1b:' + Schema.SObjectType.Account.fields.getMap().get('name').getDescribe().getLabel());`,
			debug: "T1:zz:zz:zz:zz | T1b:Account Name",
		},
		{
			name: "field token field write",
			source: `Schema.SObjectField t = Account.SObjectType.getDescribe().fields.getMap().get('name');
t.field = 'Industry';
System.debug('T2:' + t + ':' + Schema.SObjectType.Account.fields.getMap().get('name') + ':' + Schema.SObjectType.Account.fields.getMap().get('name').getDescribe().getName() + ':' + Schema.SObjectType.Account.fields.Name);`,
			debug: "T2:Industry:Industry:Industry:Industry",
		},
		{
			name: "field token via path write",
			source: `Schema.SObjectField t = Schema.SObjectType.Account.fields.Name;
t.field = 'Industry';
System.debug('T3:' + t + ':' + Schema.SObjectType.Account.fields.Name + ':' + Schema.SObjectType.Account.fields.getMap().get('name') + ':' + Schema.SObjectType.Account.fields.getMap().get('name').getDescribe().getName());`,
			debug: "T3:Industry:Industry:Industry:Industry",
		},
		{
			name: "type token write persists in the VM",
			source: `Schema.SObjectType tok = Schema.getGlobalDescribe().get('account');
tok.object = 'Contact';
System.debug('P9a:' + Schema.getGlobalDescribe().get('account') + ':' + tok);`,
			debug: "P9a:Contact:Contact",
		},
		{
			name: "describe sObjectType token write",
			source: `Schema.DescribeSObjectResult d = Widget__c.SObjectType.getDescribe();
Schema.SObjectType tok = d.getSObjectType();
tok.object = 'Contact';
System.debug('T4:' + d.getSObjectType() + ':' + Widget__c.SObjectType.getDescribe().getSObjectType() + ':' + Widget__c.SObjectType.getDescribe().getName());`,
			debug: "T4:Widget__c:Widget__c:Widget__c",
		},
		{
			name: "record type and field set maps are private copies",
			source: `Map<String, Schema.RecordTypeInfo> rts = Widget__c.SObjectType.getDescribe().getRecordTypeInfosByName();
rts.put('zz', null);
System.debug('P10a:' + rts.containsKey('zz') + ':' + Widget__c.SObjectType.getDescribe().getRecordTypeInfosByName().containsKey('zz'));
Map<String, Schema.FieldSet> fs = Schema.SObjectType.Widget__c.fieldSets.getMap();
fs.put('zz', null);
System.debug('P10b:' + fs.containsKey('zz') + ':' + Schema.SObjectType.Widget__c.fieldSets.getMap().containsKey('zz'));`,
			debug: "P10a:true:false | P10b:true:false",
		},
		{
			name: "pristine reads",
			source: `Schema.SObjectField t = Schema.SObjectType.Account.fields.getMap().get('name');
System.debug('R1:' + t + ':' + t.label + ':' + t.getDescribe().getName() + ':' + Schema.SObjectType.Account.fields.Name + ':' + Schema.getGlobalDescribe().get('account') + ':' + Schema.getGlobalDescribe().size() + ':' + Schema.SObjectType.Account.fields.getMap().size() + ':' + Widget__c.SObjectType.getDescribe().getSObjectType() + ':' + Widget__c.SObjectType.getDescribe().fields.getMap().keySet());`,
			debug: "R1:Name:Account Name:Name:Name:Account:20:62:Widget__c:{createdbyid, createddate, id, isdeleted, lastmodifiedbyid, lastmodifieddate, name, ownerid, owner__c, recordtypeid, status__c, systemmodstamp}",
		},
		{
			name: "describe renderings",
			source: `System.debug('S1:' + describeShareDigest(String.valueOf(Widget__c.SObjectType.getDescribe())));
System.debug('S2:' + describeShareDigest(String.valueOf(Schema.describeSObjects(new List<String>{'Account'})[0])));
System.debug('S3:' + describeShareDigest(String.valueOf(Schema.SObjectType.Account.fields)));
System.debug('S4:' + describeShareDigest(String.valueOf(Schema.SObjectType.Account.fields.getMap())));
System.debug('S5:' + describeShareDigest(JSON.serialize(Schema.SObjectType.Account.fields.getMap().keySet())));`,
			debug: "S1:9702/6b6bbc7e6dd5 | S2:34894/0e56d86445ef | S3:33316/7c307881fd53 | S4:7552/1f383175d47a | S5:914/9c0c26ea89c4",
		},
		{
			name: "heap size with describe roots and field maps",
			source: `Schema.DescribeSObjectResult d = Account.SObjectType.getDescribe();
Integer a = Limits.getHeapSize();
Map<String, Schema.SObjectField> m = d.fields.getMap();
Integer b = Limits.getHeapSize();
Schema.DescribeSObjectResult d2 = Widget__c.SObjectType.getDescribe();
Schema.DescribeSObjectResult d3 = Schema.describeSObjects(new List<String>{'Contact'})[0];
Integer c = Limits.getHeapSize();
Map<String, Schema.SObjectField> m2 = d3.fields.getMap();
Schema.SObjectField t = m2.get('lastname');
Integer e = Limits.getHeapSize();
System.debug('H1:' + a + ':' + b + ':' + c + ':' + e);`,
			debug: "H1:15168:20838:42104:48312",
		},
		{
			name:       "namespaced field maps",
			namespaced: true,
			source: `Schema.DescribeSObjectResult d = Schema.getGlobalDescribe().get('zzns__gadget__c').getDescribe();
Map<String, Schema.SObjectField> fm = d.fields.getMap();
System.debug('N1:' + fm.keySet() + ':' + fm.get('plain__c') + ':' + fm.get('size__c') + ':' + fm.get('zzns__plain__c') + ':' + fm.get('parent__r'));
System.debug('S2:' + describeShareDigest(String.valueOf(d)));
System.debug('N3:' + Schema.SObjectType.zzns__Gadget__c.fields.Plain__c + ':' + Schema.SObjectType.zzns__Gadget__c.fields.zzns__Size__c.getDescribe().getName() + ':' + Limits.getHeapSize());`,
			debug: "N1:{createdbyid, createddate, id, isdeleted, lastmodifiedbyid, lastmodifieddate, name, ownerid, zzns__plain__c, systemmodstamp, zzns__parent__c, zzns__size__c}:zzns__Plain__c:zzns__Size__c:zzns__Plain__c:zzns__Parent__c | S2:15393/247a581c76b7 | N3:zzns__Plain__c:zzns__Size__c:8095",
		},
		{
			name:       "namespaced field token write",
			namespaced: true,
			source: `Schema.SObjectField t = Schema.getGlobalDescribe().get('zzns__gadget__c').getDescribe().fields.getMap().get('plain__c');
t.field = 'zzns__Size__c';
System.debug('N4:' + t + ':' + Schema.SObjectType.zzns__Gadget__c.fields.getMap().get('Plain__c') + ':' + Schema.SObjectType.zzns__Gadget__c.fields.Plain__c);`,
			debug: "N4:zzns__Size__c:zzns__Size__c:zzns__Size__c",
		},
	}
}

func describeShareNamespacedOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Namespace = "zzns"
	// Match describeShareOrg's explicit Account schema, so namespace variants
	// of the shared field-map baselines describe the same Account fields.
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Account", Fields: map[string]storage.Field{"RecordTypeId": {
			APIName: "RecordTypeId", Label: "Record Type ID", Type: storage.FieldReference,
			ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType",
		}},
	}}
	configureStandardField(&org, "Account", "Division")
	for _, name := range []string{"Account", "User", "RecordType"} {
		storage.EnsureStandardObject(&org, name)
	}
	org.Objects["zzns__Gadget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "zzns__Gadget__c",
			Label:   "Gadget",
			Fields: map[string]storage.Field{
				"zzns__Size__c":   {APIName: "zzns__Size__c", Label: "Size", Type: storage.FieldDecimal},
				"Plain__c":        {APIName: "Plain__c", Label: "Plain", Type: storage.FieldString},
				"zzns__Parent__c": {APIName: "zzns__Parent__c", Label: "Parent", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	return org
}

func describeShareOrgFor(namespaced bool) storage.OrgState {
	if namespaced {
		return describeShareNamespacedOrg()
	}
	return describeShareOrg()
}

func describeShareOrg() storage.OrgState {
	org := storage.NewOrgState()
	// This local sharing fixture includes a captured RecordTypeId reference.
	// Its frozen field-map/rendering expectations describe that explicit schema.
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Account", Fields: map[string]storage.Field{"RecordTypeId": {
			APIName: "RecordTypeId", Label: "Record Type ID", Type: storage.FieldReference,
			ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType",
		}},
	}}

	configureStandardField(&org, "Account", "Division")
	for _, name := range []string{"Account", "Contact", "User", "RecordType"} {
		storage.EnsureStandardObject(&org, name)
	}
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Widget__c",
			Label:   "Widget",
			Fields: map[string]storage.Field{
				"Status__c": {APIName: "Status__c", Label: "Status", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Open", Label: "Open", Active: true}}},
				"Owner__c":  {APIName: "Owner__c", Label: "Owner", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			RecordTypes: []storage.RecordTypeInfo{{ID: storage.ID("012000000000001AAA"), Name: "Default", DeveloperName: "Default", Active: true, Available: true}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Metadata.FieldSets = append(org.Metadata.FieldSets, storage.FieldSetMetadata{ObjectName: "Widget__c", Name: "Summary", Label: "Summary", Fields: []storage.FieldSetMemberMetadata{{Field: "Status__c"}}})
	return org
}

// describeShareDigest is not Apex: rows call it through the debug text below
// so long renderings compare by digest.
func describeShareDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%d/%s", len(text), hex.EncodeToString(sum[:6]))
}

// describeShareRun compiles and runs a row on machine and returns its debug
// output and error text. Debug lines tagged S<n> carry renderings, which are
// replaced by their digest.
func describeShareRun(t *testing.T, machine *VM, row describeShareRow) (string, string) {
	t.Helper()
	source := strings.ReplaceAll(row.source, "describeShareDigest(", "(")
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatalf("%s: compile: %v", row.name, err)
	}
	result, err := machine.Execute(program)
	lines := make([]string, 0, len(result.Debug))
	for _, line := range result.Debug {
		if len(line) > 3 && line[0] == 'S' && line[2] == ':' {
			line = line[:3] + describeShareDigest(line[3:])
		}
		lines = append(lines, line)
	}
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	return strings.Join(lines, " | "), errText
}

func describeShareUnprimed() *VM {
	return describeShareUnprimedFor(false)
}

func describeShareUnprimedFor(namespaced bool) *VM {
	org := describeShareOrgFor(namespaced)
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	return machine
}

func describeSharePrimedBase(t *testing.T) (*VM, *storage.RuntimeTemplate) {
	t.Helper()
	return describeSharePrimedBaseFor(t, false)
}

func describeSharePrimedBaseFor(t *testing.T, namespaced bool) (*VM, *storage.RuntimeTemplate) {
	t.Helper()
	org := describeShareOrgFor(namespaced)
	// Match the runner: complete test-context schema before freezing it.
	// Clones must not add async objects to an already frozen generation.
	initializer := New(nil)
	initializer.SetOrg(&org)
	initializer.EnableTestContext()
	template := storage.NewRuntimeTemplate(org)
	PrimeRuntimeTemplateSchema(&template)
	base := New(nil)
	base.EnableTestContext()
	base.PrimeMetadataSchema(&template.Org)
	if strings.TrimSpace(base.metadataCacheStamp) == "" {
		t.Fatal("base schema stamp was not primed")
	}
	return base, &template
}

func describeShareClone(base *VM, template *storage.RuntimeTemplate) *VM {
	clone := base.CloneRuntime(nil)
	org := template.CloneRuntimeOrg()
	clone.SetRuntimeTemplateOrg(&org)
	clone.EnableTestContext()
	return clone
}

func describeShareCheck(t *testing.T, mode string, row describeShareRow, debug, errText string) {
	t.Helper()
	if debug != row.debug || errText != row.err {
		t.Fatalf("%s [%s]:\n debug = %q\n want    %q\n err = %q\n want  %q", row.name, mode, debug, row.debug, errText, row.err)
	}
}

// warmDescribeShare describes every object the rows touch, so later clones
// reuse describes another clone built.
const warmDescribeShareSource = `for (Schema.SObjectType t : Schema.getGlobalDescribe().values()) {
	t.getDescribe();
}
Schema.SObjectType.Account.fields.getMap();
Schema.describeSObjects(new List<String>{'Account'});`

func TestDescribeShareRowsMatchUnsharedDescribes(t *testing.T) {
	for _, row := range describeShareRows() {
		t.Run(row.name, func(t *testing.T) {
			debug, errText := describeShareRun(t, describeShareUnprimedFor(row.namespaced), row)
			describeShareCheck(t, "unprimed", row, debug, errText)

			base, template := describeSharePrimedBaseFor(t, row.namespaced)
			debug, errText = describeShareRun(t, describeShareClone(base, template), row)
			describeShareCheck(t, "first clone", row, debug, errText)
			debug, errText = describeShareRun(t, describeShareClone(base, template), row)
			describeShareCheck(t, "second clone", row, debug, errText)

			warmBase, warmTemplate := describeSharePrimedBaseFor(t, row.namespaced)
			warm := describeShareClone(warmBase, warmTemplate)
			warmProgram, err := CompileAnonymous(warmDescribeShareSource)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := warm.Execute(warmProgram); err != nil {
				t.Fatal(err)
			}
			debug, errText = describeShareRun(t, describeShareClone(warmBase, warmTemplate), row)
			describeShareCheck(t, "warm clone", row, debug, errText)
		})
	}
}

// A clone's writes to describe values, and the static state its run leaves
// behind, must not reach a sibling clone: each sibling still reads the values
// an unprimed VM reads.
func TestDescribeShareClonesIsolateWrites(t *testing.T) {
	rows := describeShareRows()
	var pristine describeShareRow
	for _, row := range rows {
		if row.name == "pristine reads" {
			pristine = row
		}
	}
	wantDebug, wantErr := describeShareRun(t, describeShareUnprimed(), pristine)
	base, template := describeSharePrimedBase(t)
	for _, row := range rows {
		if row.namespaced {
			continue
		}
		writer := describeShareClone(base, template)
		describeShareRun(t, writer, row)
		reader := describeShareClone(base, template)
		debug, errText := describeShareRun(t, reader, pristine)
		if debug != wantDebug || errText != wantErr {
			t.Fatalf("after %q in a sibling clone:\n debug = %q\n want    %q\n err = %q\n want  %q", row.name, debug, wantDebug, errText, wantErr)
		}
	}
}

// Parallel test workers describe through one shared cache. Run writers and
// readers concurrently (with -race this also checks that no shared describe
// value is written).
func TestDescribeShareConcurrentClones(t *testing.T) {
	var rows []describeShareRow
	var pristine describeShareRow
	for _, row := range describeShareRows() {
		if row.name == "pristine reads" {
			pristine = row
		}
		if !row.namespaced {
			rows = append(rows, row)
		}
	}
	wantDebug, wantErr := describeShareRun(t, describeShareUnprimed(), pristine)
	base, template := describeSharePrimedBase(t)
	clones := make([]*VM, 0, 2*len(rows))
	for range 2 * len(rows) {
		clones = append(clones, describeShareClone(base, template))
	}
	var wg sync.WaitGroup
	failures := make(chan string, len(clones))
	for i, clone := range clones {
		wg.Add(1)
		go func(i int, clone *VM) {
			defer wg.Done()
			row := rows[i%len(rows)]
			if i%2 == 1 {
				row = pristine
			}
			program, err := CompileAnonymous(strings.ReplaceAll(row.source, "describeShareDigest(", "("))
			if err != nil {
				failures <- err.Error()
				return
			}
			result, _ := clone.Execute(program)
			if i%2 == 1 && strings.Join(result.Debug, " | ") != wantDebug {
				failures <- fmt.Sprintf("concurrent pristine read = %q, want %q", strings.Join(result.Debug, " | "), wantDebug)
			}
		}(i, clone)
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
	if wantErr != "" {
		t.Fatalf("pristine row failed: %s", wantErr)
	}
}
