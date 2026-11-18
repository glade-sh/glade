@IsTest
private class T423_D20 {
  @IsTest static void run() {
    try { Integer n = Database.countQuery('SELECT COUNT() FROM Account WHERE Name = :acc.Name'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
