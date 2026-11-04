List<Account> rows = new List<Account>{
  new Account(Name='Safe', FalseFlag__c=null),
  new Account(Name='Safe', TrueFlag__c=null),
  new Account(Name='Reject', FalseFlag__c=null),
  new Account(Name='Reject', TrueFlag__c=null),
  new Account(Name='Safe'),
  new Account(Name='Reject')
};
List<String> names = new List<String>{'false_unmatched','true_unmatched','false_matched','true_matched','valid','reject'};
for (Integer i=0; i<rows.size(); i++) {
  Database.SaveResult r = Database.insert(rows[i], false);
  System.debug('CBROW|' + names[i] + '|' + (r.isSuccess() ? 'SUCCESS' : String.valueOf(r.getErrors()[0].getStatusCode())) + '|' + (r.isSuccess() ? '' : String.join(r.getErrors()[0].getFields(),',')));
}
