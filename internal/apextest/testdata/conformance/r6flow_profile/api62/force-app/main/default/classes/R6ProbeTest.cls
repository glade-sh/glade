@IsTest private class R6ProbeTest {
 @IsTest static void trueBranch() { System.assertEquals('MATCH', R6Probe.observe('R6True')); }
 @IsTest static void falseBranch() { System.assertEquals('PASS', R6Probe.observe('R6False')); }
}