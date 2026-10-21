public with sharing class MyR24 {
    public static String rd(PrbLedger__c r) {
        String s = '';
        try { Object v = r.Plan__c; s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        try { Object v = r.get('Plan__c'); s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        try { Object v = r.get(PrbLedger__c.Plan__c); s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        try { Object v = r.Plan__r; s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        try { Object v = r.Note__c; s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        try { s += r.isSet('Plan__c') ? 'T' : 'F'; } catch (Exception e) { s += 'E'; }
        return s;
    }
}