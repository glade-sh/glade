@IsTest
private class QLT_I05 {
  @IsTest static void run() {
    try { QLabels inst = new QLabels(); String loc = 'Hot'; List<Account> l = [SELECT Id FROM Account WHERE Rating = :QLabels.HOT]; } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
