@IsTest
private class T423_D40 {
  @IsTest static void run() {
    try { String Nope='x'; List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :NOPE'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
