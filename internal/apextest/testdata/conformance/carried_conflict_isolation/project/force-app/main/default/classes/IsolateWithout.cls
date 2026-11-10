public without sharing class IsolateWithout {
    public static Object observe() {
        Map<String,Object> result = new Map<String,Object>();
        result.put('profile', ProfileInventoryProbe.observe('limited'));
        List<Object> ordered = (List<Object>)ProfileInventoryProbe.observe('inventory');
        result.put('profileSortedFirst', ordered.isEmpty() ? null : ordered[0]);
        result.put('profileCount', ordered.size());
        for (String kind : new List<String>{'active','inactive','limited'}) {
            try { result.put(kind, EmailServicesProbe.observe(kind)); }
            catch (Exception e) { result.put(kind, new Map<String,Object>{'exceptionType'=>e.getTypeName(),'exceptionMessage'=>e.getMessage()}); }
        }
        return result;
    }
}
