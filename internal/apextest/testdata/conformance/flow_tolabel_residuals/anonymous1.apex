insert new List<Cedar_Report__c>{ new Cedar_Report__c(Name='m1', Cedar_Pick__c='A_Val', Cedar_Text__c='mix'), new Cedar_Report__c(Name='m2', Cedar_Pick__c='B_Val', Cedar_Text__c='mix') };
String out = 'anon ' + CedarProbe67.run() + ' || ' + CedarStatic67.run() + ' || ' + CedarStatic62.run() + ' || ' + CedarProbe.grouped();
for (String soql : new List<String>{ 'SELECT COUNT(Id) c, toLabel(Cedar_Pick__c) lbl FROM Cedar_Report__c WHERE Cedar_Text__c = \'mix\' GROUP BY Cedar_Text__c', 'SELECT COUNT(Id) c, Cedar_Pick__c FROM Cedar_Report__c GROUP BY Cedar_Text__c', 'SELECT COUNT(Id) c, toLabel(Cedar_Pick__c) lbl FROM Cedar_Report__c' }) {
    try { List<String> p = new List<String>(); for (AggregateResult a : Database.query(soql)) p.add(String.valueOf(a.getPopulatedFieldsAsMap())); p.sort(); out += ' [' + String.join(p, ';') + ']'; }
    catch (Exception e) { out += ' [EXC ' + e.getTypeName() + ': ' + e.getMessage() + ']'; }
}
System.assertEquals('SENTINEL', out);
