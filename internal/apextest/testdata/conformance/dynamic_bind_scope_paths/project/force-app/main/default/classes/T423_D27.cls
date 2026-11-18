@IsTest
private class T423_D27 {
  @IsTest static void run() {
    try { Map<String,String> m = new Map<String,String>{'k'=>'x'}; List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :m.get(\'k\')'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
