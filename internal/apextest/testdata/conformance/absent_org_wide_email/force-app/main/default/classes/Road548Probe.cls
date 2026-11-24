@IsTest private class Road548Probe {
 static String probe(Boolean direct) {
  try {
   Messaging.SingleEmailMessage m = new Messaging.SingleEmailMessage(); m.setToAddresses(new String[]{'recipient@example.invalid'}); m.setSubject('probe'); m.setPlainTextBody('probe');
   if (direct) m.setOrgWideEmailAddressId('0D2000000000001'); else { List<OrgWideEmailAddress> found = [SELECT Id FROM OrgWideEmailAddress WHERE Address = 'absent@example.invalid']; m.setOrgWideEmailAddressId(found.isEmpty() ? null : found[0].Id); }
   Messaging.SendEmailResult r = Messaging.sendEmail(new Messaging.SingleEmailMessage[]{m}, false)[0]; String out = 'success=' + r.isSuccess(); for (Messaging.SendEmailError e : r.getErrors()) out += ';error=' + e.getStatusCode() + ':' + e.getMessage(); return out;
  } catch (Exception e) { return 'exception=' + e.getTypeName() + ':' + e.getMessage(); }
 }
 @IsTest static void direct() { System.assert(false, 'ROAD548=' + probe(true)); }
 @IsTest static void lookup() { System.assert(false, 'ROAD548=' + probe(false)); }
}