@IsTest
private class T423_D17 {
  @IsTest static void run() {
    try { Account acc = new Account(Name='x'); List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :acc.Bogus__c'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
