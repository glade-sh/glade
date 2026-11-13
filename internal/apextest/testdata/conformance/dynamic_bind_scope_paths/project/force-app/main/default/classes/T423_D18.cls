@IsTest
private class T423_D18 {
  @IsTest static void run() {
    try { if (true) { Account acc = new Account(Name='x'); } List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :acc.Name'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
