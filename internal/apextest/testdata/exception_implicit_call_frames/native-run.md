started: 2026-11-13T08:29:41Z
finished: 2026-11-13T08:30:49Z
apis: 62.0 67.0 68.0
routes: anonymous istest
cases: 24
sf: @salesforce/cli/2.157.8
hub: campaign Dev Hub (scratch org, 30 days, deleted after the run)
mode: live
42 observed stacks: seven cases x three class/source APIs x anonymous and @IsTest routes.
Only the missing implicit-call cases are new captures; existing exception-stack observations were reused unchanged.
Each API has its own helper and test classes, deployed with matching class metadata API.
Anonymous: String observed = PendingProbe<api>.capture('<id>'); then System.debug of base64(observed).
Named: PendingFrameTest<api>.case_<id> intentionally fails an assertion to transport the complete caught stack.
The named test failures are expected observations, not failed captures.
org.tsv decodes the anonymous base64 and extracts the named assertion Actual value without changing stack contents.
Cases: literal control; getter argument; postfix increment getter/setter; getter receiver; first static field initialization;
new instance with initializer and getter; throwing getter boundary. All throw IllegalArgumentException.
The fresh scratch org used a 30-day duration and was deleted and verified gone after capture.

Accessor symbol spelling is outside this caller-position repair. The throwing-getter boundary
checks the getter location and complete caller stack; the native __sfdc_Broken symbol remains
recorded, while the current VM renders Broken.get.
