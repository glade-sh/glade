package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestCalendarSF216Exact(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "40.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeCalendar62Assertions.cls"), `@IsTest private class GladeCalendar62Assertions {
 private static Map<String,Object> range(Date day,List<Datetime> times,String pattern) {
  String startText=Datetime.newInstanceGMT(day,Time.newInstance(0,0,0,0)).format(pattern);
  String endText=Datetime.newInstanceGMT(day,Time.newInstance(23,59,59,0)).format(pattern);
  Datetime startValue=Datetime.valueOf(startText);
  Datetime endValue=Datetime.valueOf(endText);
  Integer count=0;
  for(Datetime value:times){if(value>=startValue && value<=endValue){count++;}}
  return new Map<String,Object>{'pattern'=>pattern,'startText'=>startText,'endText'=>endText,'startMillis'=>startValue.getTime(),'endMillis'=>endValue.getTime(),'inclusiveCount'=>count};
 }
 private static Map<String,Object> observe(Date day) {
  List<Datetime> times=new List<Datetime>();
  for(Integer iDay=-1;iDay<2;iDay++) {
   Date d=day.addDays(iDay);
   for(Integer i=0;i<24;i++){times.add(Datetime.newInstance(d,Time.newInstance(i,0,0,0)));}
  }
  return new Map<String,Object>{'day'=>String.valueOf(day),'zone'=>UserInfo.getTimeZone().getID(),'sampleCount'=>times.size(),'firstMillis'=>times[0].getTime(),'lastMillis'=>times[71].getTime(),'originalLowercase'=>range(day,times,'yyyy-MM-dd hh:mm:ss'),'uppercaseControl'=>range(day,times,'yyyy-MM-dd HH:mm:ss')};
 }
 @IsTest static void assertCalendarFormattingAndBounds() {
  Profile p=[SELECT Id FROM Profile WHERE Name='System Administrator'];
  List<Object> observations=new List<Object>();
  Integer ordinal=0;
  for(String timezone:new List<String>{'America/Los_Angeles','Australia/Sydney'}) {
   ordinal++;
   User u=new User(Alias='admin',Email='owned@example.invalid',EmailEncodingKey='UTF-8',LastName='Testing',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey=timezone,UserName='gladecalendar62.'+UserInfo.getOrganizationId()+'.'+Datetime.now().getTime()+'.'+ordinal+'@example.invalid');
   System.runAs(u) {
    observations.add(observe(Date.newInstance(2026,5,2)));
    observations.add(observe(Date.newInstance(2026,1,2)));
   }
  }
  System.assertEquals(JSON.deserializeUntyped('[{"uppercaseControl":{"inclusiveCount":24,"endMillis":1777766399000,"startMillis":1777680000000,"endText":"2026-05-02 16:59:59","startText":"2026-05-01 17:00:00","pattern":"yyyy-MM-dd HH:mm:ss"},"originalLowercase":{"inclusiveCount":24,"endMillis":1777723199000,"startMillis":1777636800000,"endText":"2026-05-02 04:59:59","startText":"2026-05-01 05:00:00","pattern":"yyyy-MM-dd hh:mm:ss"},"lastMillis":1777874400000,"firstMillis":1777618800000,"sampleCount":72,"zone":"America/Los_Angeles","day":"2026-05-02"},{"uppercaseControl":{"inclusiveCount":24,"endMillis":1767398399000,"startMillis":1767312000000,"endText":"2026-01-02 15:59:59","startText":"2026-01-01 16:00:00","pattern":"yyyy-MM-dd HH:mm:ss"},"originalLowercase":{"inclusiveCount":24,"endMillis":1767355199000,"startMillis":1767268800000,"endText":"2026-01-02 03:59:59","startText":"2026-01-01 04:00:00","pattern":"yyyy-MM-dd hh:mm:ss"},"lastMillis":1767510000000,"firstMillis":1767254400000,"sampleCount":72,"zone":"America/Los_Angeles","day":"2026-01-02"},{"uppercaseControl":{"inclusiveCount":24,"endMillis":1777766399000,"startMillis":1777680000000,"endText":"2026-05-03 09:59:59","startText":"2026-05-02 10:00:00","pattern":"yyyy-MM-dd HH:mm:ss"},"originalLowercase":{"inclusiveCount":24,"endMillis":1777766399000,"startMillis":1777680000000,"endText":"2026-05-03 09:59:59","startText":"2026-05-02 10:00:00","pattern":"yyyy-MM-dd hh:mm:ss"},"lastMillis":1777813200000,"firstMillis":1777557600000,"sampleCount":72,"zone":"Australia/Sydney","day":"2026-05-02"},{"uppercaseControl":{"inclusiveCount":24,"endMillis":1767398399000,"startMillis":1767312000000,"endText":"2026-01-03 10:59:59","startText":"2026-01-02 11:00:00","pattern":"yyyy-MM-dd HH:mm:ss"},"originalLowercase":{"inclusiveCount":24,"endMillis":1767398399000,"startMillis":1767312000000,"endText":"2026-01-03 10:59:59","startText":"2026-01-02 11:00:00","pattern":"yyyy-MM-dd hh:mm:ss"},"lastMillis":1767441600000,"firstMillis":1767186000000,"sampleCount":72,"zone":"Australia/Sydney","day":"2026-01-02"}]'),observations,'Exact SF214 bounds, strings and counts');
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeCalendar62Assertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		b, _ := json.Marshal(run)
		t.Fatalf("calendar: %s", b)
	}
}
