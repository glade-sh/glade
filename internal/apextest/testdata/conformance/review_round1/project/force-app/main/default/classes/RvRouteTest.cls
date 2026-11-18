@IsTest private class RvRouteTest {
  static void seed() { insert new HdA__c(Name='a'); insert new HdB__c(Name='b'); HdCtl.reset('plain'); }
  @IsTest static void hdPlain() { seed(); HdA__c a = [SELECT Id FROM HdA__c LIMIT 1]; a.Name='x';
    try { update a; System.assert(false, 'hdPlain|ok|d=' + HdCtl.depth); } catch (DmlException e) { System.assert(false, 'hdPlain|' + e.getTypeName() + '|d=' + HdCtl.depth + '|max=' + e.getMessage().contains('maximum trigger depth exceeded')); } }
  @IsTest static void hdCatch() { seed(); HdCtl.reset('catch'); HdA__c a = [SELECT Id FROM HdA__c LIMIT 1]; a.Name='x'; update a; System.assert(false, 'hdCatch|log=' + HdCtl.log + '|d=' + HdCtl.depth); }
  @IsTest static void hdPartial() { seed(); HdCtl.reset('partial'); HdA__c a = [SELECT Id FROM HdA__c LIMIT 1]; a.Name='x'; update a; System.assert(false, 'hdPartial|log=' + HdCtl.log + '|d=' + HdCtl.depth); }
  @IsTest static void hdSwallowLimits() { seed(); HdCtl.reset('swallow'); HdA__c a = [SELECT Id FROM HdA__c LIMIT 1]; a.Name='x'; update a; System.assert(false, 'hdSwallow|log=' + HdCtl.log + '|' + HdCtl.lim + '|after=dml' + Limits.getDmlStatements() + ',q' + Limits.getQueries()); }
  @IsTest static void nestedInner() { HiLog.nested = false; HiLog.seen = ''; HiP__c p = new HiP__c(Name='p'); insert p; insert new HiC__c(Name='a', P__c=p.Id); System.assert(false, 'nested|' + HiLog.seen); }
}