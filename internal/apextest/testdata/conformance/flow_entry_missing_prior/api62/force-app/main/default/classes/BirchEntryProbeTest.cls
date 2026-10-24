@IsTest private class BirchEntryProbeTest {
 @IsTest static void before_true() { BirchEntryProbe.emit('before',true); }
 @IsTest static void before_false() { BirchEntryProbe.emit('before',false); }
 @IsTest static void after_true() { BirchEntryProbe.emit('after',true); }
 @IsTest static void after_false() { BirchEntryProbe.emit('after',false); }
 @IsTest static void before_missing() { BirchEntryProbe.missingPrior('before'); }
 @IsTest static void after_missing() { BirchEntryProbe.missingPrior('after'); }
}
