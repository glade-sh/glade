@IsTest private class MfProbe {
  static void grant() { Id ps = [SELECT Id FROM PermissionSet WHERE Name='Mf_All'].Id; if ([SELECT count() FROM PermissionSetAssignment WHERE AssigneeId = :UserInfo.getUserId() AND PermissionSetId = :ps] == 0) { System.runAs(new User(Id=UserInfo.getUserId())) { insert new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(), PermissionSetId=ps); } } }
  @IsTest static void M1api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=-1234.5678; r.Cur__c=-1234.5; r.Pct__c=-12.5; r.Int__c=-7;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M1api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=-1234.5678; r.Cur__c=-1234.5; r.Pct__c=-12.5; r.Int__c=-7;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M2api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=1234567.891; r.Cur__c=0.5; r.Pct__c=50; r.Int__c=1000000;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M2api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=1234567.891; r.Cur__c=0.5; r.Pct__c=50; r.Int__c=1000000;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M3api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=0.0001; r.Cur__c=0; r.Pct__c=0; r.Int__c=0;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M3api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=0.0001; r.Cur__c=0; r.Pct__c=0; r.Int__c=0;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M4api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Dt__c=Datetime.newInstanceGmt(2031,3,9,15,4,5); r.D__c=Date.newInstance(2031,3,9); r.T__c=Time.newInstance(15,4,5,0);
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M4api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Dt__c=Datetime.newInstanceGmt(2031,3,9,15,4,5); r.D__c=Date.newInstance(2031,3,9); r.T__c=Time.newInstance(15,4,5,0);
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M5api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=-0.5; r.Cur__c=-0.01; r.Pct__c=-0.5; r.D__c=Date.newInstance(1999,12,31);
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M5api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=-0.5; r.Cur__c=-0.01; r.Pct__c=-0.5; r.D__c=Date.newInstance(1999,12,31);
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M6api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=99999999999999.9999; r.Cur__c=1234567890123456.78; r.Int__c=-999999999999999999L;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M6api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=99999999999999.9999; r.Cur__c=1234567890123456.78; r.Int__c=-999999999999999999L;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M7api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Dt__c=Datetime.newInstanceGmt(2031,12,31,0,0,0); r.T__c=Time.newInstance(0,0,0,0);
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M7api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Dt__c=Datetime.newInstanceGmt(2031,12,31,0,0,0); r.T__c=Time.newInstance(0,0,0,0);
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M8api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Dt__c=Datetime.newInstanceGmt(2031,1,1,23,59,59); r.T__c=Time.newInstance(23,59,59,999);
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void M8api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Dt__c=Datetime.newInstanceGmt(2031,1,1,23,59,59); r.T__c=Time.newInstance(23,59,59,999);
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R1api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=0.0005;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R1api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=0.0005;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R10api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=2.0004; r.Int__c=5;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R10api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=2.0004; r.Int__c=5;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R2api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=0.0015;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R2api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=0.0015;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R3api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=0.0025;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R3api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=0.0025;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R4api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=1.2345;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R4api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=1.2345;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R5api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=-0.0005;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R5api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=-0.0005;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R6api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=-0.0025;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R6api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=-0.0025;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R7api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=0.9995;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R7api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=0.9995;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R8api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Num__c=1234.5675;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R8api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Num__c=1234.5675;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R9api62() {
grant();
Mf__c r = new Mf__c(Name='ERR62');
r.Cur__c=0.005; r.Pct__c=0.005;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void R9api67() {
grant();
Mf__c r = new Mf__c(Name='ERR67');
r.Cur__c=0.005; r.Pct__c=0.005;
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void V1api62() {
grant();
Mf__c r = new Mf__c(Name='VAR62');
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
  @IsTest static void V1api67() {
grant();
Mf__c r = new Mf__c(Name='VAR67');
String out;
try { insert r; out = 'inserted'; } catch (DmlException e) { out = e.getDmlMessage(0); }
 System.assert(false, 'OBS ' + out);
 }
}
