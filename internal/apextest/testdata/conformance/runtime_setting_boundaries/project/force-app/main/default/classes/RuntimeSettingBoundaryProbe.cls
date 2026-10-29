@IsTest
private class RuntimeSettingBoundaryProbe {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(RuntimeSettingCases.observe()));
    }
}
