trigger ResidualLeadChange on LeadChangeEvent (after insert) {
 for (LeadChangeEvent e : Trigger.new) { ResidualLog.add(e.ChangeEventHeader, e.Company == null ? 'nc' : e.Company); }
}
