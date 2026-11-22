@IsTest private class CommunityCaseDmlTest {
 @IsTest static void observe() {
  PclSupport.setup();
  User u = PclSupport.portal('Customer Community User');
  List<String> out = new List<String>();
  System.runAs(u) {
   out.add('describe=' + Case.SObjectType.getDescribe().isAccessible() + ',' + Case.SObjectType.getDescribe().isCreateable() + ',' + Case.SObjectType.getDescribe().isUpdateable() + ',' + Case.SObjectType.getDescribe().isDeletable());
   Case c = new Case(Subject='Synthetic case', Status='New', Origin='Web');
   try { insert as user c; out.add('insert=ok'); } catch (Exception e) { out.add('insert=' + e.getTypeName()); }
   if (c.Id != null) {
    try { c.Subject='Changed'; update as user c; out.add('update=ok'); } catch (Exception e) { out.add('update=' + e.getTypeName()); }
    try { delete as user c; out.add('delete=ok'); } catch (Exception e) { out.add('delete=' + e.getTypeName()); }
   }
  }
  System.assert(false, String.join(out, '|'));
 }
}
