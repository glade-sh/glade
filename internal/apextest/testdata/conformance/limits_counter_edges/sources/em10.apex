Messaging.SingleEmailMessage m = new Messaging.SingleEmailMessage();
m.setToAddresses(new List<String>{'edge@example.com'});
m.setSubject('s');
m.setPlainTextBody('b');
for (Integer i = 0; i < 10; i++) { Messaging.sendEmail(new List<Messaging.Email>{m}); }
obs = Limits.getEmailInvocations() + '|' + Limits.getLimitEmailInvocations();
