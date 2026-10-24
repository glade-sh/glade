trigger ParcelChangeShape on Parcel__ChangeEvent (after insert) {
 System.assertEquals('Parcel__ChangeEvent',Trigger.new[0].getSObjectType().getDescribe().getName());
}
