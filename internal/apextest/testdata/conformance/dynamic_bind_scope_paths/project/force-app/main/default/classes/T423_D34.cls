@IsTest
private class T423_D34 {
  @IsTest static void run() {
    try { List<Account> l = Database.queryWithBinds('SELECT Id FROM Account WHERE Name = :nope', new Map<String,Object>(), AccessLevel.USER_MODE); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
