List<String> s = new List<String>();
for (AggregateResult r : [SELECT Amt__c am, COUNT(Id) cnt FROM Ord__c GROUP BY Amt__c]) {
    s.add(String.valueOf(r.get('am')) + ':' + String.valueOf(r.get('cnt')));
}
System.debug('OBS|groupnum|' + String.join(s, ','));
