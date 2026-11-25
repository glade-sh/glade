for (Integer i = 0; i < 2; i++) {
  try {
    Messaging.SingleEmailMessage m = new Messaging.SingleEmailMessage();
    m.setToAddresses(new String[]{'recipient@example.invalid'});
    m.setSubject('probe'); m.setPlainTextBody('probe');
    if (i == 0) m.setOrgWideEmailAddressId('0D2000000000001');
    else { List<OrgWideEmailAddress> found = [SELECT Id FROM OrgWideEmailAddress WHERE Address = 'absent@example.invalid']; m.setOrgWideEmailAddressId(found.isEmpty() ? null : found[0].Id); }
    Messaging.SendEmailResult r = Messaging.sendEmail(new Messaging.SingleEmailMessage[]{m}, false)[0];
    String out = 'success=' + r.isSuccess(); for (Messaging.SendEmailError e : r.getErrors()) out += ';error=' + e.getStatusCode() + ':' + e.getMessage();
    System.debug('ROAD548_CASE_' + i + '=' + out);
  } catch (Exception e) { System.debug('ROAD548_CASE_' + i + '=exception=' + e.getTypeName() + ':' + e.getMessage()); }
}
