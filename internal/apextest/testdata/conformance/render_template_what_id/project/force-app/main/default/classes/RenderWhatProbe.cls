public class RenderWhatProbe {
    static String err(Exception e) { return e.getTypeName().replace('System.', '') + ':' + e.getMessage(); }
    static String render(Id templateId, Id who, Id what) {
        try {
            Messaging.SingleEmailMessage m = Messaging.renderStoredEmailTemplate(templateId, who, what, Messaging.AttachmentRetrievalOption.METADATA_ONLY);
            return 'ok subject=' + m.getSubject();
        } catch (Exception e) { return err(e); }
    }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        String uniq = String.valueOf(Math.abs(Crypto.getRandomInteger()));
        EmailTemplate t = [SELECT Id FROM EmailTemplate WHERE DeveloperName = 'V35RenderText' LIMIT 1];
        Account live = new Account(Name = 'V35 Live');
        insert live;
        Account gone = new Account(Name = 'V35 Gone ' + uniq);
        insert gone;
        delete gone;
        Contact c = new Contact(LastName = 'V35R' + uniq, Email = 'v35r' + uniq + '@example.com');
        insert c;
        Contact goneContact = new Contact(LastName = 'V35G' + uniq, Email = 'v35g' + uniq + '@example.com');
        insert goneContact;
        delete goneContact;
        m.put('whatLive', render(t.Id, c.Id, live.Id));
        m.put('whatDeleted', render(t.Id, c.Id, gone.Id));
        m.put('whatDeletedNoWho', render(t.Id, null, gone.Id));
        m.put('whatNull', render(t.Id, c.Id, null));
        m.put('whoDeleted', render(t.Id, goneContact.Id, live.Id));
        m.put('whatWrongType', render(t.Id, c.Id, c.Id));
        m.put('templateNull', render(null, c.Id, live.Id));
        delete c;
        delete live;
        return m;
    }
}
