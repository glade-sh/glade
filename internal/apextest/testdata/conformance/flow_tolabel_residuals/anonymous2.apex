insert new Cedar_Report__c(Name='n1', Cedar_Pick__c='B_Val', Cedar_Text__c='mix2');
insert new Cedar_Report__c(Name='n2', Cedar_Pick__c='A_Val', Cedar_Text__c='mix2');
insert new Cedar_Report__c(Name='n3', Cedar_Pick__c='B_Val', Cedar_Text__c='mix3');
insert new Cedar_Report__c(Name='n4', Cedar_Text__c='mix3');
String out = '';
for (String soql : new List<String>{
  'SELECT COUNT(Id) c, toLabel(Cedar_Pick__c) lbl FROM Cedar_Report__c WHERE Cedar_Text__c = \'mix2\' GROUP BY Cedar_Text__c',
  'SELECT COUNT(Id) c, toLabel(Cedar_Pick__c) lbl FROM Cedar_Report__c WHERE Cedar_Text__c = \'mix3\' GROUP BY Cedar_Text__c',
  'SELECT COUNT(Id) c, toLabel(Cedar_Pick__c) lbl FROM Cedar_Report__c WHERE Cedar_Text__c = \'mix2\'',
  'SELECT Cedar_Text__c, toLabel(Cedar_Pick__c) lbl FROM Cedar_Report__c WHERE Cedar_Text__c LIKE \'mix%\' GROUP BY Cedar_Text__c',
  'SELECT Cedar_Text__c t, toLabel(Cedar_Pick__c) FROM Cedar_Report__c WHERE Cedar_Text__c LIKE \'mix%\' GROUP BY Cedar_Text__c' }) {
    try { List<String> p = new List<String>(); for (AggregateResult a : Database.query(soql)) p.add(String.valueOf(a.getPopulatedFieldsAsMap())); p.sort(); out += ' [' + String.join(p, ';') + ']'; }
    catch (Exception e) { out += ' [EXC ' + e.getTypeName() + ': ' + e.getMessage() + ']'; }
}
System.assertEquals('SENTINEL', out);
