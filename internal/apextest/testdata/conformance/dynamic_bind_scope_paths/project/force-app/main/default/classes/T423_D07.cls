@IsTest
private class T423_D07 {
  @IsTest static void run() {
    try { String nope='x'; Integer n = Database.countQuery('SELECT COUNT() FROM Account WHERE Name = :nope'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
