@IsTest private class R6UserTest {
 @IsTest static void trueBranch() { System.assertEquals('MATCH', R6Probe.observe('R6UserTrue')); }
 @IsTest static void falseBranch() { System.assertEquals('PASS', R6Probe.observe('R6UserFalse')); }
}
