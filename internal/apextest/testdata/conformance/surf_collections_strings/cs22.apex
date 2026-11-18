List<String> rows = new List<String>();
Long M = 4294967296L;
Date dd = Date.newInstance(2026, 3, 8);
Datetime dt = Datetime.newInstanceGmt(2026, 3, 8, 10, 30, 0);
Time tm = Time.newInstance(10, 30, 0, 0);
Boolean bt = true; Boolean bf = false;
rows.add('bool=' + bt.hashCode() + ',' + bf.hashCode() + '|date=' + dd.hashCode() + '|dt=' + dt.hashCode() + '|time=' + tm.hashCode());
Long l1 = 1L; Long l2 = 4294967296L; Long l3 = -1L; Long l4 = 4294967297L; Long l5 = 9223372036854775807L;
rows.add('long=' + l1.hashCode() + ',' + l2.hashCode() + ',' + l3.hashCode() + ',' + l4.hashCode() + ',' + l5.hashCode());
Integer i1 = 1; Integer i2 = -1;
rows.add('int=' + i1.hashCode() + ',' + i2.hashCode() + '|ll=' + (new List<Long>{l2}).hashCode() + '|ls=' + (new Set<Long>{l2}).hashCode());
rows.add('hcFlagT=' + new Hc__c(Flag__c=true).hashCode() + '|hcFlagF=' + new Hc__c(Flag__c=false).hashCode() + '|hcFlagNull=' + new Hc__c(Flag__c=null).hashCode());
rows.add('hcD=' + new Hc__c(D__c=dd).hashCode() + '|hcDt=' + new Hc__c(Dt__c=dt).hashCode() + '|hcNumBig=' + new Hc__c(Num__c=4294967296L).hashCode());
rows.add('acctEmpNeg=' + new Account(NumberOfEmployees=-1).hashCode());
rows.add('keys=' + 'Flag__c'.hashCode() + ',' + 'D__c'.hashCode() + ',' + 'Dt__c'.hashCode() + ',' + 'x'.hashCode());
for (String r : rows) { System.debug('OBS|' + r); }
