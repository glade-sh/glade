package vm

import (
	"fmt"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// Scheduler zone follows the documented System.schedule contract. Separate
// the scheduled instant, local construction/formatting and GMT display so a
// change to display cannot mask a changed schedule. Owned user_timezone.json
// The captured row observes typed String.valueOf in the running user zone. Retain
// separate GMT Object/concatenation/toString controls and Pacific winter/summer
// offsets without claiming gap/fold captures.
// https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_scheduler.htm
func TestScheduledTimezoneInstantAndDisplayControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, zone := range []string{"GMT", "America/Los_Angeles"} {
			for _, month := range []int{1, 6} {
				t.Run(fmt.Sprintf("%s/%s/%d", api, zone, month), func(t *testing.T) {
					machine := New(nil)
					org := storage.NewOrgState()
					machine.SetOrg(&org)
					machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue(zone)}})
					machine.EnableTestContext()
					if err := machine.RegisterClass(Class{Name: "MeridianWorker", Interfaces: []string{"Schedulable"}}); err != nil {
						t.Fatal(err)
					}
					body, err := CompileAnonymous("")
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterMethod(Method{Name: "MeridianWorker.execute", ClassName: "MeridianWorker", Params: []Param{{Name: "context", Type: "SchedulableContext"}}, Program: body}); err != nil {
						t.Fatal(err)
					}
					loc, err := time.LoadLocation(zone)
					if err != nil {
						t.Fatal(err)
					}
					instant := time.Date(2098, time.Month(month), 20, 0, 0, 0, 0, loc)
					local := instant.Format("2006-01-02 15:04:05")
					gmt := instant.UTC().Format("2006-01-02 15:04:05")
					program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Test.startTest();
Id jobId=System.schedule('Meridian job','0 0 0 20 %d ? 2098',new MeridianWorker());
Datetime localConstructed=Datetime.newInstance(2098,%d,20,0,0,0);
Datetime gmtConstructed=Datetime.newInstanceGmt(2098,%d,20,0,0,0);
for(Integer phase : new List<Integer>{0,1}) {
 if(phase==1) Test.stopTest();
 CronTrigger job=[SELECT NextFireTime FROM CronTrigger WHERE Id=:jobId];
 System.assertEquals(localConstructed.getTime(),job.NextFireTime.getTime());
 System.assertEquals(%dL,job.NextFireTime.getTime());
 System.assertEquals('%s',job.NextFireTime.format('yyyy-MM-dd HH:mm:ss'));
 System.assertEquals('%s',job.NextFireTime.formatGmt('yyyy-MM-dd HH:mm:ss'));
 System.assertEquals('%s',String.valueOf(job.NextFireTime));
 Object boxed=job.NextFireTime;
 System.assertEquals('%s',String.valueOf(boxed));
 System.assertEquals('stamp %s','stamp '+job.NextFireTime);
 System.assertEquals('%s',job.NextFireTime.toString());
 System.assertEquals(%dL,localConstructed.getTime()-gmtConstructed.getTime());
 System.debug('PHASE='+phase+'|ZONE='+UserInfo.getTimeZone().getID()+'|GMT='+job.NextFireTime.formatGmt('yyyy-MM-dd HH:mm:ss')+'|LOCAL='+job.NextFireTime.format('yyyy-MM-dd HH:mm:ss')+'|VALUEOF='+String.valueOf(job.NextFireTime));
}
`, month, month, month, instant.UnixMilli(), local, gmt, local, gmt, gmt, gmt, instant.UnixMilli()-time.Date(2098, time.Month(month), 20, 0, 0, 0, 0, time.UTC).UnixMilli()), CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					for _, line := range result.Debug {
						t.Log(line)
					}
				})
			}
		}
	}
}
