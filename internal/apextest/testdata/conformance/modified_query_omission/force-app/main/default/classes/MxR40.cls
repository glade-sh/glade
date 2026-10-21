public with sharing class MxR40 {
    public static String rd(PrbLedger__c r) {
        String s = '';
        try { Object v = r.Plan__c; s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        try { Object v = r.get('Plan__c'); s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        try { Object v = r.Plan__r; s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        try { Object v = r.Note__c; s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        return s;
    }

}