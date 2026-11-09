@IsTest private class R6LoopTest {
 @IsTest static void found() { insert new Account(Name='R6 Loop Match'); System.assertEquals('MATCH', R6LoopProbe.observe('R6 Loop Match')); }
 @IsTest static void absent() { System.assertEquals('PASS', R6LoopProbe.observe('R6 Loop Absent')); }
}
