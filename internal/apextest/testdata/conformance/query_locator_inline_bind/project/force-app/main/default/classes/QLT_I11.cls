@IsTest
private class QLT_I11 {
  @IsTest static void run() {
    try { QLabels inst = new QLabels(); String loc = 'Hot'; Database.QueryLocator ql = Database.getQueryLocator([SELECT Id FROM Account WHERE Rating = :QLabels.HOT WITH USER_MODE]); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
