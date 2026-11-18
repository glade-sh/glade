@IsTest
private class T423_D37 {
  @IsTest static void run() {
    try { try { throw new DmlException('x'); } catch (DmlException ex) { } List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :ex'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
