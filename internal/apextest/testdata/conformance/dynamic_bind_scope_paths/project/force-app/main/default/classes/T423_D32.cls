@IsTest
private class T423_D32 {
  @IsTest static void run() {
    try { List<Account> l = H423.byStatic(); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
