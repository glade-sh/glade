@IsTest
private class Hx7Test {
 @IsTest static void observe() {
List<String> rows = new List<String>();
Long M = 4294967296L;
List<Datetime> dts = new List<Datetime>{
 Datetime.newInstanceGmt(1970, 1, 1, 0, 0, 0),
 Datetime.newInstanceGmt(1970, 1, 1, 0, 0, 1),
 Datetime.newInstanceGmt(1970, 1, 1, 0, 1, 0),
 Datetime.newInstanceGmt(1970, 1, 1, 1, 0, 0),
 Datetime.newInstanceGmt(1970, 1, 2, 0, 0, 0),
 Datetime.newInstanceGmt(2026, 3, 8, 0, 0, 0),
 Datetime.newInstanceGmt(2026, 3, 8, 10, 30, 0),
 Datetime.newInstanceGmt(2026, 3, 8, 10, 30, 1),
 Datetime.newInstanceGmt(2000, 1, 1, 12, 0, 0),
 Datetime.newInstance(2026, 3, 8, 10, 30, 0),
 Datetime.valueOfGmt('2026-03-08 10:30:00'),
 Datetime.newInstance(1772965800000L)
};
for (Datetime d : dts) { rows.add('dt[' + d.getTime() + ']=' + d.hashCode() + '|hc=' + new Hc__c(Dt__c=d).hashCode()); }
List<Date> ds = new List<Date>{ Date.newInstance(1970, 1, 1), Date.newInstance(1970, 1, 2), Date.newInstance(2026, 3, 8), Date.newInstance(2026, 3, 9), Date.newInstance(2000, 1, 1), Date.newInstance(1969, 12, 31) };
for (Date d : ds) { rows.add('date[' + d + ']=' + d.hashCode() + '|hc=' + new Hc__c(D__c=d).hashCode() + '|dtGmt=' + Datetime.newInstanceGmt(d, Time.newInstance(0,0,0,0)).hashCode()); }
List<Time> ts = new List<Time>{ Time.newInstance(0,0,0,0), Time.newInstance(0,0,0,1), Time.newInstance(0,0,1,0), Time.newInstance(0,1,0,0), Time.newInstance(1,0,0,0), Time.newInstance(23,59,59,999), Time.newInstance(10,30,0,0) };
for (Time t : ts) { rows.add('time[' + t + ']=' + t.hashCode()); }
Integer ineg = -1; Integer imax = 2147483647; Integer imin = -2147483647 - 1; Long lneg = -1L; Long lsmall = 5L;
rows.add('int=' + ineg.hashCode() + ',' + imax.hashCode() + ',' + imin.hashCode() + '|long=' + lneg.hashCode() + ',' + lsmall.hashCode() + '|acctEmp=' + new Account(NumberOfEmployees=-1).hashCode() + ',' + new Account(NumberOfEmployees=imin).hashCode());
Object oi = ineg; Object ol = lneg;
rows.add('obj=' + oi.hashCode() + ',' + ol.hashCode() + '|sys=' + System.hashCode(ineg) + ',' + System.hashCode(lneg) + '|li=' + (new List<Integer>{-1}).hashCode() + '|ll=' + (new List<Long>{-1L}).hashCode());
 System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
