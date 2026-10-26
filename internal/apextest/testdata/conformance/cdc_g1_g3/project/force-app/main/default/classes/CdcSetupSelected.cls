@IsTest public class CdcSetupSelected {
    @TestSetup static void seed() { insert new Contact(LastName='setup-selected'); }
    @IsTest static void setup_selected() { CdcGapObservation.emit('setup_selected'); }
}
