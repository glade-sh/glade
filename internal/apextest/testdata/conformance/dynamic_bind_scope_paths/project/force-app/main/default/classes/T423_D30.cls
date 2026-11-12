@IsTest
private class T423_D30 {
  @IsTest static void run() {
    try { List<Account> l = H423.withParam('x'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
