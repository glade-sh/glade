@IsTest private class CaptureMixedStartBothRoleTest {
    @IsTest static void observe() { Test.startTest(); String result=CaptureMixedDml.observe(true); Test.stopTest(); System.assert(false, 'OBS|'+JSON.serialize(result)); }
}
