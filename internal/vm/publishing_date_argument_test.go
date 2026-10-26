package vm

import "testing"

// Native publishing_date_arguments rows accept both immediate (null) and
// scheduled archiving; the project replay covers the parser and semantic gate.
func TestPublishingDateArgumentDirectVM(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []struct {
			name, source string
			storedID     bool
		}{
			{"string_null_date", "KbManagement.PublishingService.archiveOnlineArticle('ka0000000000001',null);", false},
			{"typed_id_null_date", "Id articleId=(Id)'ka0000000000001'; KbManagement.PublishingService.archiveOnlineArticle(articleId,null);", false},
			{"typed_id_date", "Id articleId=(Id)'ka0000000000001'; KbManagement.PublishingService.archiveOnlineArticle(articleId,Datetime.now());", false},
			{"stored_id_date", "KbManagement.PublishingService.archiveOnlineArticle(articleId,Datetime.now());", true},
		} {
			t.Run(api+"/anonymous/"+route.name, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(route.source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				if route.storedID {
					machine.Globals["articleId"] = platformScalar("Id", "ka0000000000001")
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
