@IsTest private class MeadowNativeTest {
@IsTest static void verifyRows(){
System.assertEquals('State__c', MeadowProbe.observe('custom-column'),'custom-column');
System.assertEquals('Rating', MeadowProbe.observe('standard-column'),'standard-column');
System.assertEquals('MeadowEntry__c', MeadowProbe.observe('entity'),'entity');
System.assertEquals('true', MeadowProbe.observe('filtered-groups'),'filtered-groups');
}
}
