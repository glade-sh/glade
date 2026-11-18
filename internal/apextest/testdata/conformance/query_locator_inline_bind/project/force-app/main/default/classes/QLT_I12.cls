@IsTest
private class QLT_I12 {
  @IsTest static void run() {
    try { QLabels inst = new QLabels(); String loc = 'Hot'; Database.QueryLocator ql = Database.getQueryLocator([SELECT Id FROM Account WHERE Rating = :QLabels.HOT]); Database.QueryLocatorIterator it = ql.iterator(); Boolean h = it.hasNext(); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
