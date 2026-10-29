@IsTest private class RuntimeMissingInputProbe {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(RuntimeMissingInputCases.observe())); }
}
