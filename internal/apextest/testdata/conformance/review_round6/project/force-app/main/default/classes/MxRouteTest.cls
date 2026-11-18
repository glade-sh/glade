@IsTest private class MxRouteTest {
  static String run(String mode) { MxP__c p = new MxP__c(Name='mp'); insert p; insert new MxC__c(Name='old', P__c=p.Id, Amt__c=10, ExtId__c='old');
    MxLog.reset(mode);
    upsert new List<MxC__c>{new MxC__c(ExtId__c='old', Name='old2', P__c=p.Id, Amt__c=20), new MxC__c(ExtId__c='new', Name='new', P__c=p.Id, Amt__c=7)} MxC__c.ExtId__c;
    MxP__c f = [SELECT Cnt__c, Sm__c, Mx__c FROM MxP__c WHERE Id = :p.Id]; return MxLog.seen + 'final=' + MxLog.fmt(f); }
  @IsTest static void none() { System.assert(false, 'm_none|' + run('none')); }
  @IsTest static void nested() { System.assert(false, 'm_nested|' + run('nested')); }
}