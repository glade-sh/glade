@IsTest private class ProbeConsumerTest {
    @IsTest static void check() {
        ProbeConsumer.Request r = new ProbeConsumer.Request();
        System.assertEquals(0, r.run());
        System.assertEquals(0, r.explicitSystem());
        System.assertEquals(41, r.qualified());
        System.assertEquals(41, new ProbeContainer.Reader().own());
        System.assertEquals(0, new ProbeContainer.Reader().explicitSystem());
    }
}
