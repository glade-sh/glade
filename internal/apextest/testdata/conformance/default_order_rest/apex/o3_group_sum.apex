List<String> s = new List<String>();
for (AggregateResult r : [SELECT Grp__c, SUM(Amt__c) sm FROM Ord__c GROUP BY Grp__c]) {
    s.add(String.valueOf(r.get('Grp__c')) + ':' + String.valueOf(r.get('sm')));
}
System.debug('OBS|groupsum|' + String.join(s, ','));
