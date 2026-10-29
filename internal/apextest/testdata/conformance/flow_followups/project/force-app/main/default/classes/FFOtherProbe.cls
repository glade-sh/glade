public class FFOtherProbe {
    public static Map<String, Object> observe() {
        Map<String, Object> out = new Map<String, Object>();
        out.put('FV_name_number_collision', shadow('FF_SHADOW_PASS', 19));
        out.put('FV_explicit_record_number_twin', shadow('FF_SHADOW_PASS', 7));
        out.put('FV_explicit_record_name_twin', shadow('FF_SHADOW_OTHER', 19));
        out.put('FV_nonmatching_start', shadow('FF_SKIP_SHADOW', 19));
        out.put('CE_negative_fraction', numberError('FF_NUMBER_ERROR', -1234.50));
        out.put('CE_negative_whole', numberError('FF_NUMBER_ERROR', -1234.00));
        out.put('CE_zero', numberError('FF_NUMBER_ERROR', 0.00));
        out.put('CE_negative_zero', numberError('FF_NUMBER_ERROR', -0.00));
        out.put('CE_negative_small', numberError('FF_NUMBER_ERROR', -0.50));
        out.put('CE_positive_whole', numberError('FF_NUMBER_ERROR', 1234.00));
        out.put('CE_null', numberError('FF_NUMBER_ERROR', null));
        out.put('CE_nonmatching_start', numberError('FF_NUMBER_ACCEPT', -1234.50));
        return out;
    }
    static Map<String, Object> shadow(String recordName, Integer employees) {
        Account a = new Account(Name=recordName, NumberOfEmployees=employees);
        insert a;
        a = [SELECT Description, Site FROM Account WHERE Id=:a.Id];
        return new Map<String, Object>{'description'=>a.Description, 'site'=>a.Site};
    }
    static Map<String, Object> numberError(String recordName, Decimal revenue) {
        Account a = new Account(Name=recordName, AnnualRevenue=revenue);
        try {
            insert a;
            return new Map<String, Object>{'status'=>'SUCCESS', 'message'=>null};
        } catch (DmlException e) {
            return new Map<String, Object>{'status'=>String.valueOf(e.getDmlStatusCode(0)), 'message'=>e.getDmlMessage(0)};
        }
    }
}
