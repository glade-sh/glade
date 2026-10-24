trigger StaticFieldReceiverProbeTrigger on Account (before insert) {
    System.assertEquals('parcel', ReceiverValues.Text.trim());
}
