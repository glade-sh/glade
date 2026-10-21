@IsTest private class AccountNameProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(AccountNameProbe.observe()));
    }
}
