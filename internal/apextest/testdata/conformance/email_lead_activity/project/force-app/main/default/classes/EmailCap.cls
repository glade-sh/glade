public class EmailCap {
    public static String run(String v) {
        String u = String.valueOf(Crypto.getRandomLong()).replace('-','x'); Account a = new Account(Name = 'EmailCapAcct' + u); insert a;
        Lead l = new Lead(LastName = 'EmailCapLead' + u, Company = 'EmailCapCo' + u, Email = 'emailcap' + u + '@example.invalid'); insert l;
        Messaging.SingleEmailMessage m = new Messaging.SingleEmailMessage();
        m.setSubject('s'); m.setPlainTextBody('b');
        m.setTargetObjectId(l.Id);
        m.setSaveAsActivity(true);
        if (v == 'A') { m.setWhatId(a.Id); }
        if (v == 'B') { m.setWhatId(null); }
        if (v == 'C') { m.setWhatId(a.Id); m.setTreatTargetObjectAsRecipient(false); m.setToAddresses(new List<String>{'emailcap2@example.invalid'}); }
        if (v == 'D') { m.setTreatTargetObjectAsRecipient(false); m.setToAddresses(new List<String>{'emailcap2@example.invalid'}); }
        String out = 'EC ' + v + ' ';
        try {
            List<Messaging.SendEmailResult> rs = Messaging.sendEmail(new List<Messaging.SingleEmailMessage>{m}, false);
            out += 'n=' + rs.size() + ' ok=' + rs[0].isSuccess();
            for (Messaging.SendEmailError e : rs[0].getErrors()) { out += ' err=' + e.getStatusCode() + ':' + e.getMessage(); }
        } catch (Exception ex) { out += 'EXC ' + ex.getTypeName() + ':' + ex.getMessage(); }
        out += ' tasks=' + [SELECT COUNT() FROM Task WHERE WhoId = :l.Id];
        out += ' emailmsgs=' + [SELECT COUNT() FROM EmailMessage];
        return out;
    }
}
