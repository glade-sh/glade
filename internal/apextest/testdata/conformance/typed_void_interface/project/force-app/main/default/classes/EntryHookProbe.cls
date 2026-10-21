public class EntryHookProbe {
    public static Map<String,Object> observe(String mode) {
        OwnedEntryHook.calls = 0;
        OwnedEntryHook.seen = '';
        Map<String,Object> result = new Map<String,Object>{'mode'=>mode};
        try {
            EntryHooks.VoidHook hook = new OwnedEntryHook();
            if (mode == 'typed') {
                hook.apply(new List<OwnedEntry__c>{new OwnedEntry__c(Name='blue')});
            } else if (mode == 'erased') {
                List<SObject> rows = new List<SObject>{new OwnedEntry__c(Name='green')};
                hook.apply(rows);
            } else if (mode == 'emptyTyped') {
                hook.apply(new List<OwnedEntry__c>());
            } else if (mode == 'emptyErased') {
                hook.apply(new List<SObject>());
            } else if (mode == 'nullRows') {
                List<SObject> rows = null;
                hook.apply(rows);
            } else if (mode == 'reflected') {
                EntryHooks.VoidHook reflected = (EntryHooks.VoidHook)Type.forName('OwnedEntryHook').newInstance();
                reflected.apply(new List<SObject>{new OwnedEntry__c(Name='orange')});
            } else if (mode == 'wrongElement') {
                hook.apply(new List<SObject>{new Contact(LastName='Boundary')});
            } else if (mode == 'nonVoid') {
                EntryHooks.CountHook counter = new OwnedEntryHook();
                result.put('value', counter.count(new List<SObject>{new OwnedEntry__c(Name='red')}));
            }
            result.put('outcome', 'returned');
        } catch (Exception error) {
            result.put('outcome', 'caught');
            result.put('exceptionType', error.getTypeName());
            result.put('exceptionMessage', error.getMessage());
        }
        result.put('calls', OwnedEntryHook.calls);
        result.put('seen', OwnedEntryHook.seen);
        return result;
    }
}
