List<String> rs = new List<String>();
for (Ord__c r : [SELECT Rnk__c FROM Ord__c LIMIT 5]) {
    rs.add(String.valueOf(r.Rnk__c));
}
System.debug('OBS|limited|' + String.join(rs, ','));
