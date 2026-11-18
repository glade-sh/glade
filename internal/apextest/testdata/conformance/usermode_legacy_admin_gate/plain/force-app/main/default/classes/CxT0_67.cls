@IsTest private class CxT0_67 {
  @IsTest static void m61() {
    String s = CxLog61.go();
    Integer n = [SELECT COUNT() FROM Cx__c];
    System.assert(false, 'RESULT => ' + s + ' | rows=' + n);
  }
  @IsTest static void m62() {
    String s = CxLog62.go();
    Integer n = [SELECT COUNT() FROM Cx__c];
    System.assert(false, 'RESULT => ' + s + ' | rows=' + n);
  }
}