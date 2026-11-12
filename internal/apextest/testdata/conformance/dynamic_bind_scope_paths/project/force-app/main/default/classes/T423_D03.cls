@IsTest
private class T423_D03 {
  @IsTest static void run() {
    try { if (true) { String v='x'; } List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :v'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
