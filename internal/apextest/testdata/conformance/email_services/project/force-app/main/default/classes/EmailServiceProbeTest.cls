@IsTest private class EmailServiceProbeTest {
    @IsTest static void observeNamed() {
        System.assert(false, 'PROBE|' + JSON.serialize(EmailServiceProbe.observe()));
    }
    @IsTest static void projectRunnerActiveAndInactive() {
        System.assertEquals(2, [SELECT COUNT() FROM EmailServicesFunction WHERE FunctionName IN ('MailReceiver','MailDormant')]);
        System.assertEquals(4, [SELECT COUNT() FROM EmailServicesAddress]);
        EmailServicesAddress enabled = [SELECT LocalPart, EmailDomainName, IsActive FROM EmailServicesAddress WHERE DeveloperName='Enabled'];
        System.assertEquals('mail_receiver', enabled.LocalPart);
        System.assertEquals(true, enabled.IsActive);
        System.assert(enabled.EmailDomainName.endsWith('.salesforce.com'));
        EmailServicesAddress disabled = [SELECT IsActive FROM EmailServicesAddress WHERE DeveloperName='Disabled'];
        System.assertEquals(false, disabled.IsActive);
        EmailServicesFunction dormant = [SELECT IsActive FROM EmailServicesFunction WHERE FunctionName='MailDormant'];
        System.assertEquals(false, dormant.IsActive);
    }
}
