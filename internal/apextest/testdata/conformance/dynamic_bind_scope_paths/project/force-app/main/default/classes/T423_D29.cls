@IsTest
private class T423_D29 {
  @IsTest static void run() {
    try { String callerLocal='x'; List<Account> l = H423.run(); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
