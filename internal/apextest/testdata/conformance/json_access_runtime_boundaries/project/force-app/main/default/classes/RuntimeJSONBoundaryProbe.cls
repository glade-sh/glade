@IsTest private class RuntimeJSONBoundaryProbe {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(RuntimeJSONCases.observe())); }
}
