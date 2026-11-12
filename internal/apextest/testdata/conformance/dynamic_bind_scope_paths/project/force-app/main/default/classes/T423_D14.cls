@IsTest
private class T423_D14 {
  @IsTest static void run() {
    try { Account acc = new Account(Name='x'); List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :acc.Name'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
