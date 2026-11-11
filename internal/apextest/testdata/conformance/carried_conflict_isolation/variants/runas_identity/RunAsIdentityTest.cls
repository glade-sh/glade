@IsTest private class RunAsIdentityTest {
    @IsTest static void observe() { System.assert(false, 'CAPTURE|'+JSON.serialize(RunAsIdentity.observe())); }
}
