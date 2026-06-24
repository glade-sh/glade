package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPublicCorpusOpportunityContactRolesStandardChildRelationship(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<Opportunity> opportunities = [
      SELECT Id, (SELECT Id, ContactId FROM OpportunityContactRoles)
      FROM Opportunity
    ];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{{Name: "Opportunity"}}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "OpportunityContactRoles")
}

func TestPublicCorpusOpportunityContactRolesAfterDynamicQuery(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesOpportunityContactRoles.cls"), `
public class UsesOpportunityContactRoles {
  public void run(String soql) {
    Opportunity opp = new Opportunity (
      Name = 'Acme',
      StageName = 'Prospecting'
    );
    opp = Database.query(soql);
    Integer roleCount = opp.OpportunityContactRoles.size();
    System.assertEquals(0, opp.OpportunityContactRoles.size());
  }
}
`)
	result := analyzePublicCorpusFiles(t, root, "UsesOpportunityContactRoles.cls")

	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "OpportunityContactRoles.size")
}

func TestPublicCorpusOpportunityContactRolesWithProjectOpportunityFields(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesOpportunityContactRoles.cls"), `
public class UsesOpportunityContactRoles {
  public void run(Opportunity opp) {
    Integer roleCount = opp.OpportunityContactRoles.size();
  }
}
`)
	result := analyzePublicCorpusWithSchema(t, root, schema.Schema{Objects: []schema.Object{{
		Name: "Opportunity",
		Fields: []schema.Field{
			{Name: "Custom_Status__c", Type: "Text"},
		},
	}}}, "UsesOpportunityContactRoles.cls")

	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "OpportunityContactRoles.size")
}

func TestPublicCorpusOpportunityContactRolesInNamespacedProject(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesOpportunityContactRoles.cls"), `
public class UsesOpportunityContactRoles {
  public void run(Opportunity opp) {
    System.assertEquals(0, opp.OpportunityContactRoles.size());
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "pkg",
		ApexFiles: []string{filepath.Join(root, "UsesOpportunityContactRoles.cls")},
	}, schema.Schema{Objects: []schema.Object{{
		Name: "Opportunity",
		Fields: []schema.Field{
			{Name: "pkg__Primary_Contact__c", Type: "Lookup", ReferenceTo: []string{"Contact"}, RelationshipName: "pkg__Primary_Contact__r"},
		},
	}}})
	result := Analyze(index)

	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "OpportunityContactRoles.size")
}

func TestPublicCorpusPackageRelationshipToStandardObjectWithoutLookupMetadata(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<npe01__OppPayment__c> payments = [
      SELECT Id, npe01__Opportunity__r.AccountId, npe01__Opportunity__r.Owner.Name
      FROM npe01__OppPayment__c
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "npe01"}, schema.Schema{Objects: []schema.Object{
		{Name: "npe01__OppPayment__c"},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "npe01__Opportunity__r.AccountId")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "npe01__Opportunity__r.Owner.Name")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "npe01__Opportunity__r.AccountId")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "npe01__Opportunity__r.Owner.Name")
}

func TestPublicCorpusLocalPackageRelationshipToStandardObjectWithoutLookupMetadata(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<npe01__OppPayment__c> payments = [
      SELECT Id, Opportunity__r.AccountId
      FROM npe01__OppPayment__c
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "npe01"}, schema.Schema{Objects: []schema.Object{
		{Name: "npe01__OppPayment__c"},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "Opportunity__r.AccountId")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Opportunity__r.AccountId")
}

func TestPublicCorpusPackageRelationshipMetadataTargetCanonicalizesStandardObject(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<npe01__OppPayment__c> payments = [
      SELECT Id, npe01__Opportunity__r.AccountId
      FROM npe01__OppPayment__c
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "npe01"}, schema.Schema{Objects: []schema.Object{
		{
			Name: "npe01__OppPayment__c",
			Fields: []schema.Field{{
				Name:             "npe01__Opportunity__c",
				Type:             "Lookup",
				ReferenceTo:      []string{"Opportunity__c"},
				RelationshipName: "npe01__Opportunity__r",
			}},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "npe01__Opportunity__r.AccountId")
}

func TestPublicCorpusExternalPackageRelationshipMetadataTargetCanonicalizesStandardObject(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<npe01__OppPayment__c> payments = [
      SELECT Id, npe01__Opportunity__r.AccountId
      FROM npe01__OppPayment__c
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "pkg"}, schema.Schema{Objects: []schema.Object{
		{
			Name: "npe01__OppPayment__c",
			Fields: []schema.Field{{
				Name:             "npe01__Opportunity__c",
				Type:             "Lookup",
				ReferenceTo:      []string{"Opportunity__c"},
				RelationshipName: "npe01__Opportunity__r",
			}},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "npe01__Opportunity__r.AccountId")
}

func TestPublicCorpusPackageRelationshipEmptyProjectTargetCanonicalizesStandardObject(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<npe01__OppPayment__c> payments = [
      SELECT Id, npe01__Opportunity__r.AccountId
      FROM npe01__OppPayment__c
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "npe01"}, schema.Schema{Objects: []schema.Object{
		{Name: "npe01__Opportunity__c"},
		{
			Name: "npe01__OppPayment__c",
			Fields: []schema.Field{{
				Name:             "npe01__Opportunity__c",
				Type:             "Lookup",
				ReferenceTo:      []string{"Opportunity__c"},
				RelationshipName: "Opportunity__r",
			}},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "npe01__Opportunity__r.AccountId")
}

func TestPublicCorpusPackageRelationshipLocalNameCanonicalizesStandardObject(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<npe01__OppPayment__c> payments = [
      SELECT Id, Opportunity__r.AccountId
      FROM npe01__OppPayment__c
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "npe01"}, schema.Schema{Objects: []schema.Object{
		{
			Name: "npe01__OppPayment__c",
			Fields: []schema.Field{{
				Name:             "Opportunity__c",
				Type:             "Lookup",
				ReferenceTo:      []string{"Opportunity__c"},
				RelationshipName: "Opportunity__r",
			}},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Opportunity__r.AccountId")
}

func TestPublicCorpusPackageRelationshipPrefersProjectCustomObjectOverStandardName(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<Course_Offering__c> offerings = [
      SELECT Id, Term__r.Start_Date__c, Term__r.End_Date__c
      FROM Course_Offering__c
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "hed"}, schema.Schema{Objects: []schema.Object{
		{
			Name: "hed__Course_Offering__c",
			Fields: []schema.Field{{
				Name:             "hed__Term__c",
				Type:             "MasterDetail",
				ReferenceTo:      []string{"Term__c"},
				RelationshipName: "hed__Term__r",
			}},
		},
		{
			Name: "hed__Term__c",
			Fields: []schema.Field{
				{Name: "hed__Start_Date__c", Type: "Date"},
				{Name: "hed__End_Date__c", Type: "Date"},
			},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Term__r.Start_Date__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Term__r.End_Date__c")
}

func TestPublicCorpusExternalPackageObjectFieldNamespaceEquivalence(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<pkg__Allocation__c> allocations = [
      SELECT Id, Payment__r.npe01__Paid__c
      FROM pkg__Allocation__c
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "pkg"}, schema.Schema{Objects: []schema.Object{
		{
			Name: "pkg__Allocation__c",
			Fields: []schema.Field{{
				Name:             "pkg__Payment__c",
				Type:             "Lookup",
				ReferenceTo:      []string{"npe01__OppPayment__c"},
				RelationshipName: "pkg__Payment__r",
			}},
		},
		{
			Name: "npe01__OppPayment__c",
			Fields: []schema.Field{{
				Name: "pkg__Paid__c",
				Type: "Checkbox",
			}},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Payment__r.npe01__Paid__c")
}

func TestPublicCorpusPackageChildRelationshipWithoutLookupMetadata(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<Opportunity> opportunities = [
      SELECT Id, (SELECT Id FROM npe01__OppPayment__r)
      FROM Opportunity
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "npe01"}, schema.Schema{Objects: []schema.Object{
		{Name: "Opportunity"},
		{Name: "npe01__OppPayment__c"},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "npe01__OppPayment__r")
}

func TestPublicCorpusNamespacedQueryObjectMatchesLocalMetadata(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BatchJobScheduler.cls"), `
public class BatchJobScheduler {
  public void run() {
    for (Batch_Apex_Job__c job : [
      SELECT Id, Name, batchjobsch__Batch_Group__c
      FROM batchjobsch__Batch_Apex_Job__c
    ]) {
      Integer groupNumber = Integer.valueOf(job.Batch_Group__c);
    }
  }
}
`)
	result := analyzePublicCorpusWithSchema(t, root, schema.Schema{Objects: []schema.Object{{
		Name: "Batch_Apex_Job__c",
		Fields: []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "Name", Type: "Text"},
			{Name: "Batch_Group__c", Type: "Number"},
		},
	}}}, "BatchJobScheduler.cls")

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_OBJECT", "batchjobsch__Batch_Apex_Job__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "batchjobsch__Batch_Group__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA024", "batchjobsch__Batch_Apex_Job__c")
}
