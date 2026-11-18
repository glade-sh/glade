@IsTest private class KeyAddTest {
  @IsTest static void inTest() { KsA__c a = new KsA__c(Name='t'); insert a; Map<Id,KsA__c> m = new Map<Id,KsA__c>(new List<KsA__c>{a}); m.keySet().add(a.Id); System.assert(false, 'g_test_unreached'); }
  @IsTest static void viaClass() { KsA__c a = new KsA__c(Name='t'); insert a; KeyAdd.run(new Map<Id,KsA__c>(new List<KsA__c>{a}), a.Id); System.assert(false, 'g_class_unreached'); }
}