@IsTest
private class QLT_I07 {
  @IsTest static void run() {
    try { QLabels inst = new QLabels(); String loc = 'Hot'; List<Account> l = Database.query('SELECT Id FROM Account WHERE Rating = :QLabels.HOT'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
