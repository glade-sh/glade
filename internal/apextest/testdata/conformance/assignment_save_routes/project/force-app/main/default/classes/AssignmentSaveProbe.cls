public class AssignmentSaveProbe {
    public static String rejectAt = '';
    public static List<String> events = new List<String>();
    public static void observe(List<SObject> rows, Map<Id,SObject> oldRows, Boolean beforeEvent) {
        Integer changed = 0;
        for (SObject row : rows) {
            if (row.get('OwnerId') != oldRows.get(row.Id).get('OwnerId')) changed++;
        }
        events.add((beforeEvent ? 'before:' : 'after:') + rows.size() + ':' + changed);
        for (SObject row : rows) {
            String label = String.valueOf(row.get(row.getSObjectType() == Lead.SObjectType ? 'LastName' : 'Subject'));
            if (label == 'Reject' && row.get('OwnerId') != oldRows.get(row.Id).get('OwnerId') &&
                ((beforeEvent && rejectAt == 'before') || (!beforeEvent && rejectAt == 'after'))) {
                row.addError('Assignment owner rejected');
            }
        }
    }
    public static String run(String caseId) {
        List<String> parts = caseId.split('_');
        Boolean lead = parts[0] == 'Lead';
        Boolean updating = parts[1] == 'update';
        Boolean statement = parts[2] == 'statement';
        Boolean perRecord = parts[2] == 'setoptions' || statement;
        Boolean scalar = parts[2] == 'scalar';
        Boolean largeBatch = parts[3].startsWith('chunk');
        Boolean atomic = !largeBatch && parts[4] == 'atomic';
        Integer count = largeBatch ? 401 : (scalar ? 1 : 3);
        List<SObject> rows = new List<SObject>();
        for (Integer i=0; i<count; i++) {
            String label = (parts[3] != 'chunk' && (scalar || i==1 || i==201)) ? 'Reject' : 'Keep'+i;
            rows.add(lead ? (SObject)new Lead(LastName=label, Company='Route') : (SObject)new Case(Subject=label, Origin='Web', Status='New'));
        }
        if (updating) Database.insert(rows);
        events.clear();
        rejectAt = parts[3].replace('chunk', '');
        Database.DMLOptions options = new Database.DMLOptions();
        options.assignmentRuleHeader.useDefaultRule = true;
        options.optAllOrNone = atomic;
        if (perRecord) for (SObject row : rows) row.setOptions(options);
        Map<String,Object> observed = new Map<String,Object>();
        List<Boolean> successes = new List<Boolean>();
        List<String> errors = new List<String>();
        try {
            List<Database.SaveResult> results;
            if (statement) {
                if (updating) { update rows; } else { insert rows; }
                results = new List<Database.SaveResult>();
            } else if (scalar) {
                Database.SaveResult saved = updating ? Database.update(rows[0], options) : Database.insert(rows[0], options);
                results = new List<Database.SaveResult>{saved};
            } else if (perRecord) {
                results = updating ? Database.update(rows, atomic) : Database.insert(rows, atomic);
            } else {
                results = updating ? Database.update(rows, options) : Database.insert(rows, options);
            }
            for (Database.SaveResult saved : results) {
                successes.add(saved.isSuccess());
                if (!saved.isSuccess()) errors.add(String.valueOf(saved.getErrors()[0].getStatusCode()) + ':' + saved.getErrors()[0].getMessage());
            }
            observed.put('exception', 'none');
        } catch (DmlException e) {
            observed.put('exception', 'DmlException');
            observed.put('exceptionCode', String.valueOf(e.getDmlStatusCode(0)));
            observed.put('exceptionIndex', e.getDmlIndex(0));
        }
        List<SObject> stored = lead ? (List<SObject>)[SELECT OwnerId FROM Lead WHERE Company = 'Route'] : (List<SObject>)[SELECT OwnerId FROM Case WHERE Subject = 'Reject' OR Subject LIKE 'Keep%'];
        Integer assigned = 0;
        for (SObject row : stored) if (String.valueOf(row.get('OwnerId')).startsWith('00G')) assigned++;
        observed.put('stored', stored.size());
        observed.put('assigned', assigned);
        observed.put('events', events);
        observed.put('successes', successes);
        observed.put('errors', errors);
        return JSON.serialize(observed);
    }
}
