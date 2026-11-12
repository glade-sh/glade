@IsTest
private class T423_D31 {
  @IsTest static void run() {
    try { H423 h = new H423(); List<Account> l = h.byField(); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
