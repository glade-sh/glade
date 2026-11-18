List<String> rs = new List<String>();
for (Ord__c r : [SELECT Rnk__c FROM Ord__c WHERE Name LIKE 'K%']) {
    rs.add(String.valueOf(r.Rnk__c));
}
System.debug('OBS|namefilter|' + String.join(rs, ','));
