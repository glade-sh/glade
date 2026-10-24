@IsTest public class PathSetupObservationTest {
 @TestSetup static void seed() {insert new Account(Name='oracle-setup');}
 @IsTest static void CDC17() {PathObservation.emit('CDC17');}
}
