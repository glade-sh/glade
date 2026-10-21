@IsTest(SeeAllData=true)
private class DiskScopeProbe67Test {
  @IsTest static void lookupIsNullAfterParentDelete() {
    LookupScopeChild__c child = [
      SELECT Parent__c FROM LookupScopeChild__c
      WHERE Name = 'disk-runtime-scope-child-20261019'
      LIMIT 1
    ];
    System.assertEquals(null, child.Parent__c);
  }
}
