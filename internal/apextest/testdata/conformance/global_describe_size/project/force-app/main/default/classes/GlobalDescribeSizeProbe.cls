public class GlobalDescribeSizeProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        Map<String, Schema.SObjectType> describe = Schema.getGlobalDescribe();
        Integer keys = describe.keySet().size();
        Integer looped = 0;
        for (String key : describe.keySet()) looped++;
        Set<String> lowered = new Set<String>();
        for (String key : describe.keySet()) lowered.add(key.toLowerCase());
        result.put('sizeEqualsKeySet', describe.size() == keys);
        result.put('valuesEqualsKeySet', describe.values().size() == keys);
        result.put('loopEqualsKeySet', looped == keys);
        result.put('keysAreLowerCase', lowered.size() == keys);
        result.put('isEmpty', describe.isEmpty());
        result.put('containsAccountUpper', describe.containsKey('Account'));
        result.put('containsAccountLower', describe.containsKey('account'));
        Map<String, Schema.SObjectType> copy = new Map<String, Schema.SObjectType>(describe);
        result.put('copySizeEqualsKeys', copy.size() == keys);
        result.put('keysAboveThousand', keys > 1000);
        result.put('accountKeyPresent', describe.keySet().contains('account'));
        result.put('accountUpperKeyPresent', describe.keySet().contains('Account'));
        return result;
    }
}
