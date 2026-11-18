@IsTest
private class T423_D16 {
  @IsTest static void run() {
    try { Account acc; List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :acc.Name'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
