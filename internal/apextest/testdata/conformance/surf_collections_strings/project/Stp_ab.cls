@IsTest private class Stp_ab {
 static User mk(String loc, String n) { User u = new User(Alias='stp'+n, Email='stp@example.invalid', EmailEncodingKey='UTF-8', LastName='Stp'+n, LanguageLocaleKey='en_US', LocaleSidKey=loc, ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id, TimeZoneSidKey='GMT', Username='stp'+n+String.valueOf(Crypto.getRandomLong())+'@example.invalid'); insert u; return u; }
 @IsTest static void t() {
  User a = mk('en_US','a'); User b = mk('de_DE','b'); Id sps = [SELECT Id FROM PermissionSet WHERE Name='Stp_Access'].Id; insert new PermissionSetAssignment(AssigneeId=a.Id, PermissionSetId=sps); System.runAs(new User(Id=UserInfo.getUserId())) { } insert new PermissionSetAssignment(AssigneeId=b.Id, PermissionSetId=sps);
  Datetime fromA; Datetime fromQ; Datetime fromJ; Datetime fromF; Stp_Probe__c recA; Datetime parsedA; Datetime nowA; Datetime vA; List<Datetime> listA; Map<Datetime,Integer> mapA; Datetime arithB; Stp_Probe__c queriedA; String jsonA; Datetime fromJsonSObj;
  System.runAs(a) {
   Datetime base = Datetime.newInstance(0L);
   Stp_Probe__c r = new Stp_Probe__c(Name='s', Dt__c=base); insert r; recA = r;
   queriedA = [SELECT Id, Dt__c, CreatedDate FROM Stp_Probe__c WHERE Id=:r.Id];
   fromQ = queriedA.Dt__c; fromF = recA.Dt__c;
   fromJ = (Datetime) JSON.deserialize('"1970-01-01T00:00:00.000Z"', Datetime.class);
   jsonA = JSON.serialize(recA);
   Stp_Probe__c back = (Stp_Probe__c) JSON.deserialize(jsonA, Stp_Probe__c.class); fromJsonSObj = back.Dt__c;
   nowA = Datetime.now(); vA = Datetime.valueOf('1970-01-01 00:00:00'); parsedA = Datetime.newInstance(0L);
   listA = new List<Datetime>{ base }; mapA = new Map<Datetime,Integer>{ base => 1 };
  }
  String p = 'stp';
  System.runAs(b) {
   p += '|ownB=' + Datetime.newInstance(0L).hashCode();
   p += '|fromQ=' + fromQ.hashCode() + '|fromF=' + fromF.hashCode() + '|fromJ=' + fromJ.hashCode() + '|fromJsonSObj=' + fromJsonSObj.hashCode();
   p += '|recAfield=' + recA.Dt__c.hashCode();
   p += '|valueOfA=' + vA.hashCode() + '|listA=' + listA.hashCode() + '|mapA=' + mapA.hashCode();
   arithB = fromQ.addDays(1); p += '|arith=' + arithB.hashCode() + '|arithOwn=' + Datetime.newInstance(86400000L).hashCode();
   Stp_Probe__c again = [SELECT Id, Dt__c FROM Stp_Probe__c WHERE Id=:recA.Id]; p += '|queriedB=' + again.Dt__c.hashCode();
   p += '|newRecB=' + new Stp_Probe__c(Dt__c = Datetime.newInstance(0L)).hashCode();
   p += '|jsonB=' + ((Datetime) JSON.deserialize('"1970-01-01T00:00:00.000Z"', Datetime.class)).hashCode();
  }
  System.assert(false, p);
 }
}
