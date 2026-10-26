public class RecordMapViews {
    public static List<Contact> contacts(List<SObject> rows) { return (List<Contact>)rows; }
    public static Schema.SObjectType token(List<Contact> rows) { return rows.getSObjectType(); }
    public static Map<String,Object> observe() {
        Map<String,Object> out = new Map<String,Object>();
        List<SObject> raw = new List<SObject>{new Account(Id='001000000000001AAA',Name='Meadow')};
        List<Contact> view = contacts(raw);
        out.put('viewToken',String.valueOf(view.getSObjectType()));
        Map<Id,Contact> idMap;
        try {
            idMap = new Map<Id,Contact>(view);
            out.put('idConstructor','accepted');
            out.put('idSize',idMap.size());
            out.put('idContains',idMap.containsKey('001000000000001AAA'));
            out.put('idToken',String.valueOf(idMap.getSObjectType()));
        } catch(Exception ex) { out.put('idConstructor',ex.getTypeName()); }
        try { Object item=idMap.get('001000000000001AAA'); out.put('idGet','accepted'); }
        catch(Exception ex) { out.put('idGet',ex.getTypeName()); }
        try { List<Contact> items=idMap.values(); out.put('idValuesSize',items.size()); out.put('idValuesToken',String.valueOf(items.getSObjectType())); }
        catch(Exception ex) { out.put('idValues',ex.getTypeName()); }
        try { new Map<Id,Contact>(contacts(raw)); out.put('helperConstructor','accepted'); }
        catch(Exception ex) { out.put('helperConstructor',ex.getTypeName()); }
        try { new Map<String,Contact>(view); out.put('stringConstructor','accepted'); }
        catch(Exception ex) { out.put('stringConstructor',ex.getTypeName()); }
        try { new Map<String,Contact>(contacts(raw)); out.put('stringHelperConstructor','accepted'); }
        catch(Exception ex) { out.put('stringHelperConstructor',ex.getTypeName()); }
        Map<Id,Contact> appended=new Map<Id,Contact>();
        try { appended.putAll(view); out.put('idPutAll','accepted'); }
        catch(Exception ex) { out.put('idPutAll',ex.getTypeName()); }
        out.put('idPutAllSize',appended.size());
        Map<String,Contact> stringAppended=new Map<String,Contact>();
        try { stringAppended.putAll(view); out.put('stringPutAll','accepted'); }
        catch(Exception ex) { out.put('stringPutAll',ex.getTypeName()); }
        out.put('stringPutAllSize',stringAppended.size());
        List<SObject> matching=new List<SObject>{new Contact(Id='003000000000001AAA',LastName='Brook')};
        Map<Id,Contact> control=new Map<Id,Contact>(contacts(matching));
        out.put('matchingGetType',String.valueOf(control.get('003000000000001AAA').getSObjectType()));
        out.put('matchingGetName',control.get('003000000000001AAA').LastName);
        out.put('matchingViewToken',String.valueOf(contacts(matching).getSObjectType()));
        out.put('emptyViewToken',String.valueOf(contacts(new List<SObject>()).getSObjectType()));
        out.put('inlineViewToken',String.valueOf(((List<Contact>)raw).getSObjectType()));
        out.put('relayViewToken',String.valueOf(token(view)));
        List<SObject> concrete=new List<Contact>{new Contact(LastName='Concrete')};
        out.put('concreteControlToken',String.valueOf(concrete.getSObjectType()));
        return out;
    }
}
