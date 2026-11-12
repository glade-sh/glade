@IsTest
private class T423_D01 {
  @IsTest static void run() {
    try { List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :nope'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
