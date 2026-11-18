@IsTest private class CxT1_67 {
  @TestSetup static void s() {
    Id ps = [SELECT Id FROM PermissionSet WHERE Name = 'Cx_Grant'].Id;
    insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = ps);
  }
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