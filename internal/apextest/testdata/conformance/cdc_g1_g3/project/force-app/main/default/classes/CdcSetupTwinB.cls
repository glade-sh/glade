@IsTest public class CdcSetupTwinB {
    @TestSetup static void seed() { insert new Account(Name='setup-b'); }
    @IsTest static void setup_b_first() { CdcGapObservation.emit('setup_b_first'); }
}
