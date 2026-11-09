@IsTest private class Case159CommunityTest {
 @IsTest static void pageUrls() {
  UserRole role=new UserRole(Name='Synthetic Owner Role'); insert role;
  Profile adminProfile=[SELECT Id FROM Profile WHERE Name='System Administrator' LIMIT 1];
  String uniqueName='case159'+String.valueOf(DateTime.now().getTime())+'@example.invalid';
  User owner=new User(Alias='r159own',Email='case159@example.invalid',EmailEncodingKey='UTF-8',LastName='Owner',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=adminProfile.Id,TimeZoneSidKey='America/Los_Angeles',UserName=uniqueName,UserRoleId=role.Id);
  insert owner;
  System.runAs(owner) {
   Account account=new Account(Name='Synthetic'); insert account;
   Contact contact=new Contact(LastName='Synthetic',AccountId=account.Id); insert contact;
   Profile community=[SELECT Id FROM Profile WHERE Name='Customer Community User' LIMIT 1];
   User member=new User(Alias='r159usr',Email='case159@example.invalid',EmailEncodingKey='UTF-8',LastName='Member',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=community.Id,ContactId=contact.Id,TimeZoneSidKey='America/Los_Angeles',UserName='member'+uniqueName);
   insert member;
   Test.setCurrentPage(Page.RoadMixedPage);
   String before=Page.RoadMixedPage.getUrl();
   System.runAs(member) {
    System.assertEquals('/apex/roadmixedpage', Page.RoadMixedPage.getUrl(), 'token');
    System.assertEquals('/apex/roadmixedpage', ApexPages.currentPage().getUrl(), 'current');
    System.assertEquals('/apex/RoadMixedPage', new PageReference('/apex/RoadMixedPage').getUrl(), 'explicit');
    System.assertEquals('/apex/roadmixedpage', before, 'stored');
   }
  }
 }
}
