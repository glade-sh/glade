@IsTest
private class QLT_I02 {
  @IsTest static void run() {
    try { QLabels inst = new QLabels(); String loc = 'Hot'; Database.QueryLocator ql = Database.getQueryLocator([SELECT Id FROM Account WHERE Rating = :QLabels.s]); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
