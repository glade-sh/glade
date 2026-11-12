@IsTest
private class T423_D19 {
  @IsTest static void run() {
    try { Account acc = new Account(Name='x'); Integer n = Database.countQuery('SELECT COUNT() FROM Account WHERE Name = :acc.Name'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
