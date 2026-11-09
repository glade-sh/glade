package apextest

import "testing"

// capi2_argument_prefixes/org.tsv records the accepted parameter prefixes.
func TestCapi2ArgumentAcceptedPrefixes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			p.writeClass("AcceptedPrefixTest", `@IsTest private class AcceptedPrefixTest {
   @IsTest static void probe() {
    String msg='';
    try {ConnectApi.Announcements.deleteAnnouncement(null, '0Bt000000000001');} catch(Exception e) {msg=e.getMessage();}
    System.assert(!msg.contains('Illegal value for parameter: \'announcementId\''),msg);
    msg='';
    try {ConnectApi.Chatter.deleteSubscription(null, '0E8000000000001');} catch(Exception e) {msg=e.getMessage();}
    System.assert(!msg.contains('Illegal value for parameter: \'subscriptionId\''),msg);
    msg='';
    try {ConnectApi.Datacloud.getOrder('09F000000000001');} catch(Exception e) {msg=e.getMessage();}
    System.assert(!msg.contains('Illegal value for parameter: \'orderId\''),msg);
   }
  }`)
			r := p.run()
			if s := r.Summary(); s.Total != 1 || s.Passed != 1 {
				t.Fatalf("summary=%#v problem=%q", s, firstRunProblem(r))
			}
		})
	}
}
