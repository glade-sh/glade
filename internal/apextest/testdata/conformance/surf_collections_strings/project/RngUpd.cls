@IsTest private class RngUpd {
 @IsTest static void t() {
 String p = 'update';
 try { Rng_Probe__c r = new Rng_Probe__c(Name='r0', Dt__c=Datetime.newInstanceGmt(2000,1,1,0,0,0)); insert r; r.Dt__c=Datetime.newInstanceGmt(1582,10,15,0,0,0); update r; p += '|1582-10-15 00:00:00=ok'; } catch (Exception e) { p += '|1582-10-15 00:00:00=' + e.getMessage(); }
 try { Rng_Probe__c r = new Rng_Probe__c(Name='r1', Dt__c=Datetime.newInstanceGmt(2000,1,1,0,0,0)); insert r; r.Dt__c=Datetime.newInstanceGmt(1699,12,31,23,59,59); update r; p += '|1699-12-31 23:59:59=ok'; } catch (Exception e) { p += '|1699-12-31 23:59:59=' + e.getMessage(); }
 try { Rng_Probe__c r = new Rng_Probe__c(Name='r2', Dt__c=Datetime.newInstanceGmt(2000,1,1,0,0,0)); insert r; r.Dt__c=Datetime.newInstanceGmt(1700,1,1,0,0,0); update r; p += '|1700-01-01 00:00:00=ok'; } catch (Exception e) { p += '|1700-01-01 00:00:00=' + e.getMessage(); }
 try { Rng_Probe__c r = new Rng_Probe__c(Name='r3', Dt__c=Datetime.newInstanceGmt(2000,1,1,0,0,0)); insert r; r.Dt__c=Datetime.newInstanceGmt(1700,6,15,12,0,0); update r; p += '|1700-06-15 12:00:00=ok'; } catch (Exception e) { p += '|1700-06-15 12:00:00=' + e.getMessage(); }
 try { Rng_Probe__c r = new Rng_Probe__c(Name='r4', Dt__c=Datetime.newInstanceGmt(2000,1,1,0,0,0)); insert r; r.Dt__c=Datetime.newInstanceGmt(1850,1,1,0,0,0); update r; p += '|1850-01-01 00:00:00=ok'; } catch (Exception e) { p += '|1850-01-01 00:00:00=' + e.getMessage(); }
 try { Rng_Probe__c r = new Rng_Probe__c(Name='r5', Dt__c=Datetime.newInstanceGmt(2000,1,1,0,0,0)); insert r; r.Dt__c=Datetime.newInstanceGmt(3999,12,31,23,59,59); update r; p += '|3999-12-31 23:59:59=ok'; } catch (Exception e) { p += '|3999-12-31 23:59:59=' + e.getMessage(); }
 try { Rng_Probe__c r = new Rng_Probe__c(Name='r6', Dt__c=Datetime.newInstanceGmt(2000,1,1,0,0,0)); insert r; r.Dt__c=Datetime.newInstanceGmt(4000,12,30,23,59,59); update r; p += '|4000-12-30 23:59:59=ok'; } catch (Exception e) { p += '|4000-12-30 23:59:59=' + e.getMessage(); }
 try { Rng_Probe__c r = new Rng_Probe__c(Name='r7', Dt__c=Datetime.newInstanceGmt(2000,1,1,0,0,0)); insert r; r.Dt__c=Datetime.newInstanceGmt(4000,12,31,0,0,0); update r; p += '|4000-12-31 00:00:00=ok'; } catch (Exception e) { p += '|4000-12-31 00:00:00=' + e.getMessage(); }
 try { Rng_Probe__c r = new Rng_Probe__c(Name='r8', Dt__c=Datetime.newInstanceGmt(2000,1,1,0,0,0)); insert r; r.Dt__c=Datetime.newInstanceGmt(4000,12,31,0,0,1); update r; p += '|4000-12-31 00:00:01=ok'; } catch (Exception e) { p += '|4000-12-31 00:00:01=' + e.getMessage(); }
 try { Rng_Probe__c r = new Rng_Probe__c(Name='r9', Dt__c=Datetime.newInstanceGmt(2000,1,1,0,0,0)); insert r; r.Dt__c=Datetime.newInstanceGmt(4000,12,31,23,59,59); update r; p += '|4000-12-31 23:59:59=ok'; } catch (Exception e) { p += '|4000-12-31 23:59:59=' + e.getMessage(); }
 try { Rng_Probe__c r = new Rng_Probe__c(Name='r10', Dt__c=Datetime.newInstanceGmt(2000,1,1,0,0,0)); insert r; r.Dt__c=Datetime.newInstanceGmt(4001,1,1,0,0,0); update r; p += '|4001-01-01 00:00:00=ok'; } catch (Exception e) { p += '|4001-01-01 00:00:00=' + e.getMessage(); }
 System.assert(false, p);
 }
}
