@IsTest private class StaticFieldReceiverProbeTest {
    @IsTest static void observe() { System.assertEquals('parcel', ReceiverValues.Text.trim()); StaticFieldReceiverProbe.observe(); insert new Account(Name = 'Receiver'); }
}
