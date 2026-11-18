trigger ResidualParcelChange on Parcel__ChangeEvent (after insert) {
 for (Parcel__ChangeEvent e : Trigger.new) { ResidualLog.add(e.ChangeEventHeader, e.Tag__c == null ? 'nt' : e.Tag__c); }
}
