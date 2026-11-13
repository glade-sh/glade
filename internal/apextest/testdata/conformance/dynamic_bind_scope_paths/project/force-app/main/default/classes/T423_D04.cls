@IsTest
private class T423_D04 {
  @IsTest static void run() {
    try { for (Integer i=0;i<1;i++) { String v='x'; } List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :v'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
