@IsTest
private class R5PbProbe {
  private static String run(Boolean touchStd) {
    String msg = '';
    try {
      if (touchStd) { Id sid = Test.getStandardPricebookId(); }
      Product2 p = new Product2(Name='pb-probe', IsActive=true);
      insert p;
      msg = 'pricebooks=' + [SELECT COUNT() FROM Pricebook2] + ',entries=' + [SELECT COUNT() FROM PricebookEntry WHERE Product2Id = :p.Id];
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    return msg;
  }
  @IsTest static void plain() { System.assert(false, 'R5|pb|' + run(false)); }
  @IsTest static void touched() { System.assert(false, 'R5|pb|' + run(true)); }
}
