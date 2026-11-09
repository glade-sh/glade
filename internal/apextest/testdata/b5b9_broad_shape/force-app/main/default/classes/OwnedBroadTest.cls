@IsTest private class OwnedBroadTest {
    @IsTest static void probe() {
        OwnedBroad instance = new OwnedBroad();
        Schema.Group groupValue = new Schema.Group();
        System.assertEquals(groupValue,instance.consume(new Schema.Account(),new Schema.Contact(),new Schema.User(),groupValue));
        System.assertEquals('owned',OwnedBroad.readStatic());
        System.assertEquals(2,OwnedBroad.Phase.values().size());
    }
}
