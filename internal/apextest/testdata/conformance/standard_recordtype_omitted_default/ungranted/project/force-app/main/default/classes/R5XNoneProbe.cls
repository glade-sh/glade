@IsTest
private class R5XNoneProbe {
  @IsTest static void x_none2() {
    List<String> r5out = new List<String>();
try {
  X__c x = new X__c(Name='a'); insert x;
  X__c q = [SELECT RecordTypeId, P__c FROM X__c WHERE Id=:x.Id];
  r5out.add('R5|x_none2|P=' + q.P__c + '|rtNull=' + (q.RecordTypeId == null) + '|inMemRtNull=' + (x.RecordTypeId == null) + '|inMemP=' + x.P__c);
} catch (Exception e) { r5out.add('R5|x_none2|EXC|' + e.getMessage()); }
    System.assert(false, String.join(r5out, ';'));
  }
}
