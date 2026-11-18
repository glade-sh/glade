trigger ResidualLeadDml on Lead (before insert) {
 for (Lead l : Trigger.new) { if (l.Company == 'TrigDup') l.Email = 'dup@probe.test'; }
}
