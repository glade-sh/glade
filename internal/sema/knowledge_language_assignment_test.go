package sema

import "testing"

// Owned named deployments distinguish concrete article Language assignment from
// the read-only base article view at APIs 52, 62, 67 and 68.
func TestKnowledgeArticleLanguageAssignmentBoundary(t *testing.T) {
	for _, api := range []string{"52.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			for _, test := range []struct {
				name, body string
				wantError  bool
			}{
				{"concrete-assignment", `public static String observe(Knowledge__kav article) { article.Language = 'en_US'; return article.Language; }`, false},
				{"concrete-constructor", `public static Knowledge__kav observe() { return new Knowledge__kav(Title = 'Owned', UrlName = 'owned', Language = 'en_US'); }`, false},
				{"base-assignment", `public static String observe(KnowledgeArticleVersion article) { article.Language = 'en_US'; return article.Language; }`, true},
			} {
				t.Run(test.name, func(t *testing.T) {
					result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"ArticleLanguageProbe.cls": "public class ArticleLanguageProbe {" + test.body + "}"}, api)
					if result.HasErrors() != test.wantError {
						t.Fatalf("errors = %v, want %v: %#v", result.HasErrors(), test.wantError, result.Diagnostics)
					}
				})
			}
		})
	}
}
