@isTest private class PclLeadCcTest {
    @isTest static void run() { PclSupport.setup(); User u = PclSupport.portal('Customer Community User'); System.runAs(new User(Id = UserInfo.getUserId())) { PclLeadGrant.grant(u); } PclSupport.observe('leadCc', u); }
}
