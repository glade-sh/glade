public class CedarReadOnlyProbe {
    public static Boolean enabled = false;
    public static Boolean typed = false;
    public static String operation;
    public static String phase;
    public static Map<String, String> observed = new Map<String, String>();
    public static void action(Contact record, List<Contact> records) {
        if (operation == 'put') record.put('FirstName', 'Changed');
        if (operation == 'assign') record.FirstName = 'Changed';
        if (operation == 'putSObject') record.putSObject('Account', new Account(Name='Parent'));
        if (operation == 'addError') record.addError('blocked');
        if (operation == 'listAdd') records.add(new Contact(LastName='Extra'));
    }
    public static void observe(Contact record, List<Contact> records) {
        String key = phase + '/' + operation + '/' + (typed ? 'typed' : 'generic');
        if (typed) {
            try { action(record, records); observed.put(key, 'OK'); }
            catch (System.FinalException e) { observed.put(key, 'FinalException|' + e.getTypeName() + '|' + e.getMessage()); }
            catch (System.SObjectException e) { observed.put(key, 'SObjectException|' + e.getTypeName() + '|' + e.getMessage()); }
            catch (System.NullPointerException e) { observed.put(key, 'NullPointerException|' + e.getTypeName() + '|' + e.getMessage()); }
            catch (Exception e) { observed.put(key, 'fallback|' + e.getTypeName() + '|' + e.getMessage()); }
        } else {
            try { action(record, records); observed.put(key, 'OK'); }
            catch (Exception e) { observed.put(key, 'Exception|' + e.getTypeName() + '|' + e.getMessage()); }
        }
    }
}
