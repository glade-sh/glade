package sema

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestDocumentTemplateContentDocumentQueryShape(t *testing.T) {
	// Salesforce Help's community-user setup example queries these three ID
	// fields. Its group-benefits setup example also queries Id and Name.
	// https://help.salesforce.com/s/articleView?id=ind.doc_gen_run_apex_code_for_omnistudio_foundation_customer_community_user_395891.htm&type=5
	// https://help.salesforce.com/s/articleView?id=sf.hc_admin_group_benefits_app_document_generation.htm&type=5
	result := analyzeQueryProbeWithProject(t, `public class TemplateQuery {
  public List<DocumentTemplateContentDoc> find(Id templateId) {
    List<DocumentTemplateContentDoc> documents = [
      SELECT Id, Name, DocumentTemplateId, ContentDocumentId, LatestContentVersionId
      FROM DocumentTemplateContentDoc WHERE DocumentTemplateId = :templateId
      ORDER BY LastModifiedDate LIMIT 1
    ];
    Map<String, DocumentTemplateContentDoc> byName = new Map<String, DocumentTemplateContentDoc>();
    for (DocumentTemplateContentDoc document : documents) {
      Id contentId = document.ContentDocumentId;
      Id versionId = document.LatestContentVersionId;
      byName.put(document.Name, document);
    }
    return documents;
  }
}`, typesys.ProjectInfo{SourceAPIVersion: "67.0"}, queryDiagnosticSchema())
	if result.HasErrors() {
		t.Fatalf("documented query shape rejected: %#v", result.Diagnostics)
	}
}

func TestDocumentTemplateShapeKeepsUnknownTypeAndFieldErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, code, fragment string
	}{
		{"unknownType", "MissingTemplateShape document;", "GLADESEMA006", "MissingTemplateShape"},
		{"unknownField", "List<Account> rows = [SELECT InventedField FROM Account];", "GLADESEMA_QUERY_FIELD", "InventedField"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeQueryProbeWithProject(t,
				"public class TemplateControl { void run() { "+tc.body+" } }",
				typesys.ProjectInfo{SourceAPIVersion: "67.0"}, queryDiagnosticSchema())
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Code == tc.code && strings.Contains(diagnostic.Message, tc.fragment) {
					return
				}
			}
			t.Fatalf("missing expected %s diagnostic for %s: %#v", tc.code, tc.fragment, result.Diagnostics)
		})
	}
}
