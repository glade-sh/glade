@IsTest private class Road548Shapes { static String probe(Integer i) { try {
 Messaging.SingleEmailMessage m = new Messaging.SingleEmailMessage(); m.setToAddresses(new String[]{'recipient@example.invalid'}); m.setSubject('probe'); m.setPlainTextBody('probe');
 m.setOrgWideEmailAddressId(i == 0 ? '0D2000000000001CAA' : '0D2000000000001');
 if (i == 2) m.setTargetObjectId(UserInfo.getUserId());
 if (i == 3) m.setTemplateId('00X000000000001');
 List<Messaging.SingleEmailMessage> msgs = new List<Messaging.SingleEmailMessage>{m};
 if (i == 1) { Messaging.SingleEmailMessage n = new Messaging.SingleEmailMessage(); n.setToAddresses(new String[]{'recipient@example.invalid'}); n.setSubject('probe'); n.setPlainTextBody('probe'); msgs.add(n); }
 List<Messaging.SendEmailResult> results=Messaging.sendEmail(msgs,false); String out=''; for (Messaging.SendEmailResult r:results) { if (out != '') out += '/'; out += 'success='+r.isSuccess(); for (Messaging.SendEmailError e:r.getErrors()) out += ';error='+e.getStatusCode()+':'+e.getMessage(); } return out; } catch (Exception e) { return 'exception='+e.getTypeName()+':'+e.getMessage(); } }
@IsTest static void shape0() { System.assert(false, 'ROAD548='+probe(0)); }
@IsTest static void shape1() { System.assert(false, 'ROAD548='+probe(1)); }
@IsTest static void shape2() { System.assert(false, 'ROAD548='+probe(2)); }
@IsTest static void shape3() { System.assert(false, 'ROAD548='+probe(3)); }
}