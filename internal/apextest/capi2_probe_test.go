package apextest

import "testing"

// capi2_subtype/org.tsv:2-4: the discriminator appears in JSON, not on the object.
func TestCapi2OutputSubtypeProbe(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			p.writeClass("CapiSubtypeTest", `@IsTest private class CapiSubtypeTest {
    @IsTest static void probe() {
     ConnectApi.TextSegment s = new ConnectApi.TextSegment(); s.text = 'hi';
     ConnectApi.LinkSegment l = new ConnectApi.LinkSegment();
     ConnectApi.FeedBody b = new ConnectApi.FeedBody();
     Map<String,Object> text = (Map<String,Object>) JSON.deserializeUntyped(JSON.serialize(s));
     Map<String,Object> link = (Map<String,Object>) JSON.deserializeUntyped(JSON.serialize(l));
     Map<String,Object> body = (Map<String,Object>) JSON.deserializeUntyped(JSON.serialize(b));
     System.assertEquals('Text', text.get('type'));
     System.assertEquals('Link', link.get('type'));
     System.assertEquals(null, s.type);
     System.assertEquals(false, body.containsKey('type'));
    }
   }`)
			r := p.run()
			if s := r.Summary(); s.Total != 1 || s.Passed != 1 {
				t.Fatalf("summary=%#v problem=%q", s, firstRunProblem(r))
			}
		})
	}
}

// capi2_segments/org.tsv:2-7 covers every direct MessageSegment subtype,
// including the non-instantiable ComplexSegment and explicitly assigned type.
func TestCapi2SegmentSubtypeFamily(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			p.writeClass("CapiSegmentsTest", `@IsTest private class CapiSegmentsTest {
    @IsTest static void probe() {
     Map<String,String> cases = new Map<String,String>{'EntityLinkSegment'=>'EntityLink','FieldChangeNameSegment'=>'FieldChangeName','FieldChangeValueSegment'=>'FieldChangeValue','HashtagSegment'=>'Hashtag','InlineImageSegment'=>'InlineImage','LinkSegment'=>'Link','MarkupBeginSegment'=>'MarkupBegin','MarkupEndSegment'=>'MarkupEnd','MentionSegment'=>'Mention','MoreChangesSegment'=>'MoreChanges','ResourceLinkSegment'=>'ResourceLink','TextSegment'=>'Text'};
     for(String name : cases.keySet()) {
      Object item = Type.forName('ConnectApi.'+name).newInstance();
      Map<String,Object> data = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(item));
      System.assertEquals(cases.get(name), data.get('type'), name);
     }
     ConnectApi.TextSegment s = new ConnectApi.TextSegment(); s.type = ConnectApi.MessageSegmentType.Mention;
     Map<String,Object> data = (Map<String,Object>) JSON.deserializeUntyped(JSON.serialize(s,true));
     System.assertEquals('Text', data.get('type'));
     System.assertEquals(ConnectApi.MessageSegmentType.Mention, s.type);
     List<Object> nested = (List<Object>)JSON.deserializeUntyped(JSON.serialize(new List<ConnectApi.MessageSegment>{s}));
     System.assertEquals('Text', ((Map<String,Object>)nested[0]).get('type'));
    }
   }`)
			r := p.run()
			if s := r.Summary(); s.Total != 1 || s.Passed != 1 {
				t.Fatalf("summary=%#v problem=%q", s, firstRunProblem(r))
			}
		})
	}
}

// Existing native gap all.Announcements.deleteAnnouncement(String,String), API 67.0.
func TestCapi2AnnouncementUserIDProbe(t *testing.T) {
	p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	p.writeClass("CapiAnnouncementTest", `@IsTest private class CapiAnnouncementTest {
  @IsTest static void probe() {
   try { ConnectApi.Announcements.deleteAnnouncement(null, UserInfo.getUserId()); System.assert(false, 'expected argument failure'); }
   catch (ConnectApi.ConnectApiException e) { System.assert(e.getMessage().startsWith('Illegal value for parameter: \'announcementId\':')); }
  }
 }`)
	r := p.run()
	if s := r.Summary(); s.Total != 1 || s.Passed != 1 {
		t.Fatalf("summary=%#v problem=%q", s, firstRunProblem(r))
	}
}
