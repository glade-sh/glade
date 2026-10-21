@IsTest private class NotQueryEdgeControlsTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|'+JSON.serialize(NotQueryEdgeControls.observe()));
    }
}
