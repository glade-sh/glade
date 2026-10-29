@IsTest private class RuntimeMessageBoundaryProbe {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(RuntimeMessageCases.observe())); }
}
