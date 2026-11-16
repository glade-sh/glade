List<String> rows = new List<String>();
Long M = 4294967296L;
rows.add('user=' + UserInfo.getLocale() + '|' + UserInfo.getTimeZone().getID());
List<Datetime> dts = new List<Datetime>{
 Datetime.newInstanceGmt(1970, 1, 1, 0, 0, 0),
 Datetime.newInstanceGmt(1970, 1, 1, 0, 0, 1),
 Datetime.newInstanceGmt(1970, 1, 1, 0, 1, 0),
 Datetime.newInstanceGmt(1970, 1, 1, 1, 0, 0),
 Datetime.newInstance(1772965800123L),
 Datetime.newInstance(2026, 3, 8, 10, 30, 0)
};
for (Datetime d : dts) { rows.add('dt[' + d.getTime() + ']=' + d.hashCode() + '|hc=' + new Hc__c(Dt__c=d).hashCode()); }
Date dd = Date.newInstance(2026, 3, 8); Time tm = Time.newInstance(10, 30, 0, 0);
rows.add('date=' + dd.hashCode() + '|time=' + tm.hashCode());
for (String r : rows) { System.debug('OBS|' + r); }
