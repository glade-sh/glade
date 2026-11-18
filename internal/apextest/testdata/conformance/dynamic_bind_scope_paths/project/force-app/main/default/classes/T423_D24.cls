@IsTest
private class T423_D24 {
  @IsTest static void run() {
    try { for (String s : new List<String>()) { List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :s'); } } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
