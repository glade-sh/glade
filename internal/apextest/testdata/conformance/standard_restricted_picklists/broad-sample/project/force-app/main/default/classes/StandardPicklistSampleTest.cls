@IsTest
private class StandardPicklistSampleTest {
    @IsTest static void observe() { System.assert(false, 'OBS|'+JSON.serialize(StandardPicklistSample.observe())); }
}
