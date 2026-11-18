List<String> rows = new List<String>();
Long M = 4294967296L;
List<String> dv = new List<String>{'1.5','1.50','0.00','0','-2.25','1E+3','1000','0.1','12345678901234567890','100'};
for (String s : dv) {
 Decimal d = Decimal.valueOf(s);
 rows.add('dec[' + s + ']=' + d + '|scale=' + d.scale() + '|hash=' + d.hashCode() + '|hc=' + new Hc__c(Dec__c=d).hashCode() + '|acct=' + new Account(AnnualRevenue=d).hashCode());
}
Decimal lit150 = 1.50; Decimal lit15 = 1.5; Decimal lit0 = 0.00;
rows.add('lit150=' + lit150.hashCode() + '|scale=' + lit150.scale() + ';lit15=' + lit15.hashCode() + '|scale=' + lit15.scale() + ';lit0=' + lit0.hashCode() + '|scale=' + lit0.scale());
Decimal third = Decimal.valueOf(1).divide(Decimal.valueOf(3), 5);
Decimal third2 = (Decimal.valueOf(1) / Decimal.valueOf(3)).setScale(2);
Decimal third3 = Decimal.valueOf(1) / Decimal.valueOf(3);
rows.add('third5=' + third + '|' + third.scale() + '|' + third.hashCode() + ';third2=' + third2 + '|' + third2.scale() + '|' + third2.hashCode() + ';thirdRaw=' + third3 + '|' + third3.scale() + '|' + third3.hashCode());
Hc__c q = new Hc__c(Name='q', Dec__c=1.5);
insert q;
Hc__c rec = [SELECT Id, Name, Dec__c FROM Hc__c WHERE Id = :q.Id];
Decimal r = rec.Dec__c;
Long hq = rec.hashCode();
Long idE = 'Id'.hashCode() ^ String.valueOf(rec.Id).hashCode();
Long exp = (Long)('Name'.hashCode() ^ 'q'.hashCode()) + (Long)(new Hc__c(Dec__c=r).hashCode());
rows.add('soql=' + r + '|scale=' + r.scale() + '|hash=' + r.hashCode() + '|ctor=' + new Hc__c(Dec__c=r).hashCode() + '|recMatch=' + (Math.mod(Math.mod(hq - idE - exp, M) + M, M) == 0));
Decimal setv = 2.25; Set<Decimal> ds = new Set<Decimal>{1.5, 1.50}; List<Decimal> dl = new List<Decimal>{1.5, 1.50};
rows.add('set=' + ds.size() + '|listHash=' + dl.hashCode() + '|setHash=' + ds.hashCode() + '|eq=' + (Decimal.valueOf('1.5') == Decimal.valueOf('1.50')));
for (String r : rows) { System.debug('OBS|' + r); }
