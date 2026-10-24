@IsTest private class RouteTwinProbeTest {
 @IsTest static void observe() { Test.startTest(); RouteTwinProbe.submit(); Test.stopTest(); System.assert(false,'OBS|'+JSON.serialize(RouteTwinProbe.observe())); }
 @IsTest static void observeScheduledBatch() { Test.startTest(); RouteTwinProbe.submitScheduledBatch(); Test.stopTest(); System.assert(false,'OBS|'+JSON.serialize(RouteTwinProbe.observe())); }
}
