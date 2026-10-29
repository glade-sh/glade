public class UpdateMessageProbe {
    public static String norm(String text, List<Contact> rows) {
        for (Integer i = 0; i < rows.size(); i++) {
            if (rows[i].Id != null) text = text.replace(String.valueOf(rows[i].Id), '<id' + i + '>');
        }
        return text;
    }
    public static Map<String,Object> observe(String kind, String route, Boolean isBulk) {
        Contact existing = new Contact(LastName='Duplicate seed', Email='duplicate@example.invalid');
        insert existing;
        List<Contact> rows = new List<Contact>();
        if (isBulk) rows.add(new Contact(LastName='Good', Email='good@example.invalid'));
        rows.add(new Contact(LastName='Bad', Email='bad@example.invalid'));
        insert rows;
        Integer bad = rows.size() - 1;
        for (Contact row : rows) row.Description = 'changed';
        if (kind == 'validation') rows[bad].Title = 'reject-validation';
        if (kind == 'trigger') rows[bad].Title = 'reject-trigger';
        if (kind == 'field') rows[bad].Title = 'reject-field';
        if (kind == 'required') rows[bad].LastName = null;
        if (kind == 'duplicate') rows[bad].Email = existing.Email;
        if (kind == 'missingid') rows[bad] = new Contact(LastName='No id');
        Map<String,Object> out = new Map<String,Object>();
        try {
            if (route == 'statement') {
                if (isBulk) { update rows; } else { update rows[0]; }
            } else {
                List<Database.SaveResult> results;
                if (isBulk) {
                    if (route == 'default') results = Database.update(rows);
                    else results = Database.update(rows, route != 'partial');
                } else {
                    Database.SaveResult single;
                    if (route == 'default') single = Database.update(rows[0]);
                    else single = Database.update(rows[0], route != 'partial');
                    results = new List<Database.SaveResult>{single};
                }
                List<Object> observed = new List<Object>();
                for (Integer i = 0; i < results.size(); i++) {
                    Database.SaveResult sr = results[i];
                    Map<String,Object> detail = new Map<String,Object>{'success'=>sr.isSuccess(), 'id'=>sr.getId() == null ? null : '<id' + i + '>'};
                    List<Object> errors = new List<Object>();
                    for (Database.Error error : sr.getErrors()) errors.add(new Map<String,Object>{'status'=>String.valueOf(error.getStatusCode()), 'message'=>error.getMessage(), 'fields'=>error.getFields()});
                    detail.put('errors',errors); observed.add(detail);
                }
                out.put('results',observed);
            }
            out.put('thrown',false);
        } catch (DmlException e) {
            out.put('thrown',true); out.put('message',norm(e.getMessage(), rows));
            List<Object> errors = new List<Object>();
            for (Integer i = 0; i < e.getNumDml(); i++) errors.add(new Map<String,Object>{'index'=>e.getDmlIndex(i), 'id'=>e.getDmlId(i) == null ? null : '<id' + e.getDmlIndex(i) + '>', 'status'=>String.valueOf(e.getDmlStatusCode(i)), 'message'=>e.getDmlMessage(i), 'fields'=>e.getDmlFieldNames(i)});
            out.put('errors',errors);
        }
        Set<Id> ids = new Set<Id>();
        for (Contact row : rows) if (row.Id != null) ids.add(row.Id);
        Integer saved = 0;
        for (Contact row : [SELECT Description FROM Contact WHERE Id IN :ids]) if (row.Description == 'changed') saved++;
        out.put('saved',saved);
        return out;
    }
}
