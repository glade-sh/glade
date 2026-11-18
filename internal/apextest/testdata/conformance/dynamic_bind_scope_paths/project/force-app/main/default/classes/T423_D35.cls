@IsTest
private class T423_D35 {
  @IsTest static void run() {
    try { Integer n = Database.countQueryWithBinds('SELECT COUNT() FROM Account WHERE Name = :nope', new Map<String,Object>{'nope'=>'x'}, AccessLevel.USER_MODE); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
