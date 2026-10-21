@IsTest private class PageUrlSliceTest {
 @IsTest static void opaqueQuery() {
  String address='https://example.test/auth?client_id=***synthetic_client***&state=done';
  PageReference page=new PageReference(address);
  System.assert(false,'OBSERVED|opaqueQuery|'+JSON.serialize(new Map<String,Object>{'url'=>page.getUrl(),'containsMarker'=>page.getUrl().containsIgnoreCase('***synthetic_client***')}));
 }
 @IsTest static void parameterQuery() {
  PageReference page=new PageReference('/apex/Grove');
  page.getParameters().put('client_id','***synthetic_client***');
  System.assert(false,'OBSERVED|parameterQuery|'+JSON.serialize(new Map<String,Object>{'url'=>page.getUrl(),'containsMarker'=>page.getUrl().containsIgnoreCase('***synthetic_client***')}));
 }
}
