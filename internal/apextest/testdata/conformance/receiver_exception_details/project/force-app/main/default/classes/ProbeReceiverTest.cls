@IsTest private class ProbeReceiverTest {
    @IsTest static void controls() {
        System.assertEquals('value:ready', ProbeReceiver.observe(true));
        System.assertEquals('caught:System.NullPointerException|Attempt to de-reference a null object|5', ProbeReceiver.observe(false));
    }
}
