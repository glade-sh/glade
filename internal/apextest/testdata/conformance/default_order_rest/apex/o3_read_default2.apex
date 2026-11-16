List<String> rs = new List<String>();
for (Ord__c r : [SELECT Rnk__c FROM Ord__c]) {
    rs.add(String.valueOf(r.Rnk__c));
}
System.debug('OBS|default2|' + String.join(rs, ','));
