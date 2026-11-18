List<String> s = new List<String>();
for (AggregateResult r : [SELECT Grp__c, COUNT(Id) cnt FROM Ord__c WHERE Name LIKE 'K%' GROUP BY Grp__c]) {
    s.add(String.valueOf(r.get('Grp__c')) + ':' + String.valueOf(r.get('cnt')));
}
System.debug('OBS|groupfiltered|' + String.join(s, ','));
