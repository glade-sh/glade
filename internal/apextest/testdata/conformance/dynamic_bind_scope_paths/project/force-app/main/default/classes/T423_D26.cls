@IsTest
private class T423_D26 {
  @IsTest static void run() {
    try { List<String> xs = new List<String>{'x'}; List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :xs[0]'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
