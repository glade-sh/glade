trigger Road548Trigger on Account (before insert) {
for (Account a : Trigger.new) { try {
Messaging.SingleEmailMessage m = new Messaging.SingleEmailMessage(); m.setToAddresses(new String[]{'recipient@example.invalid'}); m.setSubject('probe'); m.setPlainTextBody('probe'); m.setOrgWideEmailAddressId('0D2000000000001');
Messaging.SendEmailResult r = Messaging.sendEmail(new Messaging.SingleEmailMessage[]{m}, false)[0]; String out = 'success=' + r.isSuccess(); for (Messaging.SendEmailError e : r.getErrors()) out += ';error=' + e.getStatusCode() + ':' + e.getMessage(); a.Description=out;
} catch (Exception e) { a.Description='exception=' + e.getTypeName() + ':' + e.getMessage(); } } }