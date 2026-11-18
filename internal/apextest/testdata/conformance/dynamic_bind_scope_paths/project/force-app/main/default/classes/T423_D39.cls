@IsTest
private class T423_D39 {
  @IsTest static void run() {
    try { String o1='x'; if (true) { String i1='y'; } List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :o1'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
