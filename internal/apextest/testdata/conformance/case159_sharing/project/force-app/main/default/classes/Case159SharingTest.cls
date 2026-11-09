@IsTest private class Case159SharingTest {
 private with sharing class WithSharing {
  Integer users(Id target) { return [SELECT count() FROM User WHERE Id=:target]; }
  Integer cases(Id target) { return [SELECT count() FROM Case WHERE Id=:target]; }
 }
 private without sharing class WithoutSharing {
  Integer users(Id target) { return [SELECT count() FROM User WHERE Id=:target]; }
  Integer cases(Id target) { return [SELECT count() FROM Case WHERE Id=:target]; }
 }
 private inherited sharing class InheritedSharing {
  Integer users(Id target) { return [SELECT count() FROM User WHERE Id=:target]; }
 }
 static User createUser(String profileName, String suffix) {
  Profile p=[SELECT Id FROM Profile WHERE Name=:profileName LIMIT 1];
  User u=new User(Alias=suffix,Email='synthetic@example.invalid',EmailEncodingKey='UTF-8',LastName='Synthetic',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',UserName=suffix+String.valueOf(DateTime.now().getTime())+'@example.invalid'); insert u; return u;
 }
 @IsTest static void userVisibility() {
  User a=createUser('Minimum Access - Salesforce','r159min'); User b=createUser('Standard User','r159std'); Map<String,Object> result=new Map<String,Object>();
  System.runAs(a) { result.put('with',new WithSharing().users(b.Id)); result.put('without',new WithoutSharing().users(b.Id)); result.put('inherited',new InheritedSharing().users(b.Id)); result.put('readable',Schema.SObjectType.User.isAccessible()); }
  System.assert(false,'OBSERVED|userVisibility|'+JSON.serialize(result));
 }
 @IsTest static void caseVisibility() {
  User a=createUser('Standard Platform User','r159plt'); User b=createUser('Standard User','r159std'); Map<String,Object> result=new Map<String,Object>();
  System.runAs(b) { Case c=new Case(Subject='Synthetic'); insert c; System.runAs(a) { try { result.put('with',new WithSharing().cases(c.Id)); } catch(Exception e) { result.put('with',e.getTypeName()); } try { result.put('without',new WithoutSharing().cases(c.Id)); } catch(Exception e) { result.put('without',e.getTypeName()); } result.put('readable',Schema.SObjectType.Case.isAccessible()); try { Database.query('SELECT Id FROM Case WHERE Id = :c.Id',AccessLevel.USER_MODE); result.put('userMode','ok'); } catch(Exception e) { result.put('userMode',e.getTypeName()); } } }
  System.assert(false,'OBSERVED|caseVisibility|'+JSON.serialize(result));
 }
}
