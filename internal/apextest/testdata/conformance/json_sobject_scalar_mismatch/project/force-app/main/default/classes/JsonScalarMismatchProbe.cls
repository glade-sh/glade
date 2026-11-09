public with sharing class JsonScalarMismatchProbe {
    public static Map<String, String> observe() {
        Map<String, String> got = new Map<String, String>();
        try { JSON.deserialize('{"Name":"x","NumberOfEmployees":"abc"}', Account.class); got.put('int_str','ok'); } catch (Exception e) { got.put('int_str', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"NumberOfEmployees":"abc","Name":"x"}', Account.class); got.put('int_str_first','ok'); } catch (Exception e) { got.put('int_str_first', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Name":"x","NumberOfEmployees":"1.5"}', Account.class); got.put('int_decstr','ok'); } catch (Exception e) { got.put('int_decstr', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Name":"x","NumberOfEmployees":true}', Account.class); got.put('int_true','ok'); } catch (Exception e) { got.put('int_true', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserializeStrict('{"Name":"x","NumberOfEmployees":false}', Account.class); got.put('int_false_strict','ok'); } catch (Exception e) { got.put('int_false_strict', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('[{"Name":"a"},{"Name":"x","NumberOfEmployees":"abc"}]', List<Account>.class); got.put('int_list','ok'); } catch (Exception e) { got.put('int_list', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"attributes":{"type":"Account"},"Name":"x","NumberOfEmployees":"abc"}', SObject.class); got.put('int_generic','ok'); } catch (Exception e) { got.put('int_generic', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Name":"x","CreatedDate":"abc"}', Account.class); got.put('dt_str','ok'); } catch (Exception e) { got.put('dt_str', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Name":"x","CreatedDate":"2025-01-31 06:00:00.000 UTC"}', Account.class); got.put('dt_utc','ok'); } catch (Exception e) { got.put('dt_utc', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Name":"x","CreatedDate":1.5}', Account.class); got.put('dt_float','ok'); } catch (Exception e) { got.put('dt_float', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Subject":"x","ReminderDateTime":"abc"}', Task.class); got.put('dt_task','ok'); } catch (Exception e) { got.put('dt_task', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"attributes":{"type":"Account"},"Name":"x","CreatedDate":"abc"}', SObject.class); got.put('dt_generic','ok'); } catch (Exception e) { got.put('dt_generic', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Name":"x","Probability":"abc"}', Opportunity.class); got.put('pct_str','ok'); } catch (Exception e) { got.put('pct_str', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Name":"x","Probability":true}', Opportunity.class); got.put('pct_true','ok'); } catch (Exception e) { got.put('pct_true', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Name":"x","TotalOpportunityQuantity":"abc"}', Opportunity.class); got.put('dbl_str','ok'); } catch (Exception e) { got.put('dbl_str', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Name":"x","Amount":true}', Opportunity.class); got.put('cur_true','ok'); } catch (Exception e) { got.put('cur_true', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Name":"x","IsPrivate":"abc"}', Opportunity.class); got.put('bool_str','ok'); } catch (Exception e) { got.put('bool_str', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"Name":"x","IsPrivate":"TRUE"}', Opportunity.class); got.put('bool_upper','ok'); } catch (Exception e) { got.put('bool_upper', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"LastName":"x","Birthdate":true}', Contact.class); got.put('date_true','ok'); } catch (Exception e) { got.put('date_true', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('{"LastName":"x","Birthdate":"garbage"}', Contact.class); got.put('date_str','ok'); } catch (Exception e) { got.put('date_str', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserializeStrict('{"Name":"x","NumberOfEmployees":"abc"}', Account.class); got.put('int_strict_strict','ok'); } catch (Exception e) { got.put('int_strict_strict', e.getTypeName() + '|' + e.getMessage()); }
        return got;
    }
}
