package apextest

import "testing"

func TestCapi2AssignedOutputType(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			p.writeClass("AssignedOutputTest", `@IsTest private class AssignedOutputTest { @IsTest static void probe() {
   ConnectApi.CurrencyRecordField a=new ConnectApi.CurrencyRecordField(); a.type='wrong';
   ConnectApi.ContentHubRepository b=new ConnectApi.ContentHubRepository(); b.type='wrong';
   Map<String,Object> x=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(a));
   Map<String,Object> y=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(b));
   System.assertEquals('Currency', x.get('type'));System.assertEquals('ContentHubRepository',y.get('type'));
   System.assertEquals('wrong',a.type);System.assertEquals('wrong',b.type);
  } }`)
			run := p.run()
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
				t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
			}
		})
	}
}
