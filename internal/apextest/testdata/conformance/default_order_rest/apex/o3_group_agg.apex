List<String> s = new List<String>();
for (AggregateResult r : [SELECT Grp__c grp, COUNT(Id) cnt FROM Ord__c GROUP BY Grp__c]) {
    s.add(String.valueOf(r.get('grp')) + ':' + String.valueOf(r.get('cnt')));
}
System.debug('OBS|groupagg|' + String.join(s, ','));
