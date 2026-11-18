List<String> s = new List<String>();
for (AggregateResult r : [SELECT Grp__c FROM Ord__c GROUP BY Grp__c]) {
    s.add(String.valueOf(r.get('Grp__c')));
}
System.debug('OBS|groupplain|' + String.join(s, ','));
