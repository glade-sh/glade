trigger AccountChangeShape on AccountChangeEvent (after insert) {
 System.assertEquals('AccountChangeEvent',Trigger.new[0].getSObjectType().getDescribe().getName());
}
