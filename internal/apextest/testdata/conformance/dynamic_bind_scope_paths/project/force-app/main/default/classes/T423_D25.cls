@IsTest
private class T423_D25 {
  @IsTest static void run() {
    try { List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :late'); String late='x'; } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
