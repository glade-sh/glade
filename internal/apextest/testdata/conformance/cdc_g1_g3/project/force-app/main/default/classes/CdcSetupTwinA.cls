@IsTest public class CdcSetupTwinA {
    @TestSetup static void seed() { insert new Account(Name='setup-a'); }
    @IsTest static void setup_a_first() { CdcGapObservation.emit('setup_a_first'); }
    @IsTest static void setup_a_second() { CdcGapObservation.emit('setup_a_second'); }
}
