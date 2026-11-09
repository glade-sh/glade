public class FormulaDefaultProbe {
    static String err(Exception e) { return e.getTypeName().replace('System.', '') + ':' + e.getMessage(); }
    public static Map<String,Object> observe() {
        Map<String,Object> o = new Map<String,Object>();
        SObject d = Schema.FormulaDefaultProbe__c.SObjectType.newSObject(null, true);
        o.put('loadedLabel', d.get('Label__c'));
        o.put('loadedPlain', d.get('Plain__c'));
        for (String f : new List<String>{ 'F1__c', 'F2__c', 'F3__c', 'F4__c', 'F5__c', 'F6__c', 'F7__c', 'F8__c' }) { Object v = d.get(f); o.put('loaded' + f, v == null ? null : '[' + v + ']'); }
        d.put('Issuer__c', 'Visa'); d.put('Num__c', '1111');
        o.put('afterPutLabel', d.get('Label__c'));
        SObject n = Schema.FormulaDefaultProbe__c.SObjectType.newSObject();
        o.put('plainCtorLabel', n.get('Label__c'));
        FormulaDefaultProbe__c c = new FormulaDefaultProbe__c();
        o.put('newLabel', c.Label__c);
        try { SelectOption so = new SelectOption(UserInfo.getUserId(), (String) d.get('Label__c')); o.put('selectOption', 'ok'); } catch (Exception e) { o.put('selectOption', err(e)); }
        SObject l = Schema.FormulaDefaultProbe__c.SObjectType.newSObject(null, true);
        try { SelectOption so = new SelectOption(UserInfo.getUserId(), (String) l.get('Label__c')); o.put('selectOptionLoaded', 'ok'); } catch (Exception e) { o.put('selectOptionLoaded', err(e)); }
        return o;
    }
}
