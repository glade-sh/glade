@IsTest private class CustomPlatformDescribeProbeTest {
    @TestSetup static void setup() {
        insert new LinkProbe__c(Name = 'Describe probe');
    }
    @IsTest static void describeRows() {
        System.assert(false, 'DESCRIBE_ROWS|' + JSON.serialize(CustomPlatformDescribeProbe.describeRows()));
    }
    @IsTest static void queryRows() {
        System.assert(false, 'QUERY_ROWS|' + JSON.serialize(CustomPlatformDescribeProbe.queryRows()));
    }
}
