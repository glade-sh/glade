@IsTest private class NotQueryControlsTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|'+JSON.serialize(NotQueryControls.observe()));
    }
}
