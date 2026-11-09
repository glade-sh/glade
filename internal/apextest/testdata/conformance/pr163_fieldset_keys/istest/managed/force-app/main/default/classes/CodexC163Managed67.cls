@IsTest private class CodexC163Managed67 {
 @IsTest static void verify() {
  Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
  System.assertEquals(1, sets.size());
  System.assert(sets.keySet().contains('samplepkg__codexc163capture'));
  System.assert(sets.containsKey('SAMPLEPKG__CODEXC163CAPTURE'));
  System.assertNotEquals(null, sets.get('samplepkg__CodexC163Capture'));
  System.assertEquals(false, sets.containsKey('CodexC163Capture'));
System.assertEquals(null, sets.get('CodexC163Capture'));
  Map<String, Schema.FieldSet> second = Account.SObjectType.getDescribe().fieldSets.getMap();
  sets.clear();
  System.assertEquals(0, sets.size());
  System.assertEquals(false, sets.containsKey('samplepkg__CodexC163Capture'));
  System.assertEquals(null, sets.get('samplepkg__CodexC163Capture'));
  System.assertEquals(1, second.size());
  System.assert(second.containsKey('samplepkg__CodexC163Capture'));
  Map<String, Schema.FieldSet> fresh = Account.SObjectType.getDescribe().fieldSets.getMap();
  System.assertEquals(1, fresh.size());
  System.assert(fresh.containsKey('samplepkg__CodexC163Capture'));
 }
}
