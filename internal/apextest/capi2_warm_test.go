package apextest

import "testing"

// Repeated project loads exercise the same captured siloed and JSON paths.
func TestCapi2ProjectColdAndWarm(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			p.writeClass("CapiWarmTest", `@IsTest private class CapiWarmTest { @IsTest static void probe() {
   ConnectApi.TextSegment s=new ConnectApi.TextSegment();
   Map<String,Object> obj=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(s));
   System.assertEquals('Text',obj.get('type'));
   try {ConnectApi.Announcements.deleteAnnouncement(null,UserInfo.getUserId());System.assert(false,'expected argument failure');}
   catch(ConnectApi.ConnectApiException e){System.assert(e.getMessage().startsWith('Illegal value for parameter: \'announcementId\':'));}
  }}`)
			for i := 0; i < 2; i++ {
				r := p.run()
				if s := r.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("run=%d summary=%#v problem=%q", i, s, firstRunProblem(r))
				}
			}
		})
	}
}
