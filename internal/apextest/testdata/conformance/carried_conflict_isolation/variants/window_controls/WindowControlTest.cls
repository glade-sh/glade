@IsTest private class WindowControlTest {
    @IsTest static void observe() { System.assert(false, 'CAPTURE|'+JSON.serialize(WindowControl.observe())); }
}
