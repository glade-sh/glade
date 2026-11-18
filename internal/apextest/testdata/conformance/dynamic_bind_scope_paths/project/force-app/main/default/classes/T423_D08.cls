@IsTest
private class T423_D08 {
  @IsTest static void run() {
    try { if (true) { String v='x'; } Integer n = Database.countQuery('SELECT COUNT() FROM Account WHERE Name = :v'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
