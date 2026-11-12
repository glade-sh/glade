@IsTest
private class T423_D36 {
  @IsTest static void run() {
    try { Database.QueryLocator ql = Database.getQueryLocatorWithBinds('SELECT Id FROM Account WHERE Name = :nope', new Map<String,Object>{'nope'=>'x'}, AccessLevel.USER_MODE); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
