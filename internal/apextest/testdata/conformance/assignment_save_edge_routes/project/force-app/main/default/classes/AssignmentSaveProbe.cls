public class AssignmentSaveProbe {
    public static String rejectAt = '';
    public static List<String> events = new List<String>();
    public static void observe(List<SObject> rows, Map<Id,SObject> oldRows, Boolean beforeEvent) {
        Integer changed = 0;
        for (SObject row : rows) {
            if (oldRows != null && row.get('OwnerId') != oldRows.get(row.Id).get('OwnerId')) changed++;
        }
        events.add((beforeEvent ? 'before:' : 'after:') + rows.size() + ':' + changed);
        if (rejectAt=='retry' && beforeEvent && changed>0) rows[0].addError('Assignment owner rejected');
        for (SObject row : rows) {
            String label = String.valueOf(row.get(row.getSObjectType() == Lead.SObjectType ? 'LastName' : 'Subject'));
            if (label == 'OuterReject' && rejectAt.startsWith('outer') && (beforeEvent != rejectAt.startsWith('outerpost'))) row.addError('Outer row rejected');
            String phase = rejectAt.replace('mixed', '').replace('outerpost', '').replace('outer', '');
            if (label == 'Reject' && oldRows != null && row.get('OwnerId') != oldRows.get(row.Id).get('OwnerId') &&
                ((beforeEvent && phase == 'before') || (!beforeEvent && phase == 'after'))) {
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
        Boolean mixed = parts[3].startsWith('mixed');
        Boolean outerFailure = parts[3].startsWith('outer');
        Integer count = scalar ? 1 : (outerFailure || parts[3]=='retry' ? 4 : 3);
        List<SObject> rows = new List<SObject>();
        for (Integer i=0; i<count; i++) {
            String label = (scalar || i==(outerFailure ? 2 : 1)) ? 'Reject' : 'Keep'+i;
            if (outerFailure && i==0) label = 'OuterReject';
            Boolean unmatched = parts[3]=='unmatched' || (mixed && i==0) || (outerFailure && i==1);
            rows.add(lead ? (SObject)new Lead(LastName=label, Company=unmatched ? 'Other' : 'Route') : (SObject)new Case(Subject=label, Origin=unmatched ? 'Phone' : 'Web', Status='New'));
        }
        if (parts[3]=='alreadyowned') {
            Id queueId = [SELECT Id FROM Group WHERE DeveloperName = 'AssignmentSaveQueue' LIMIT 1].Id;
            for (SObject row : rows) row.put('OwnerId', queueId);
        }
        if (updating) Database.insert(rows);
        events.clear();
        rejectAt = parts[3].replace('chunk', '');
        Database.DMLOptions options = new Database.DMLOptions();
        options.assignmentRuleHeader.useDefaultRule = parts[3]!='disabled' && parts[3]!='unknown';
        if (parts[3]=='unknown') options.assignmentRuleHeader.assignmentRuleId = '01Q000000000001';
        options.optAllOrNone = atomic;
        if (perRecord) for (Integer i=0; i<rows.size(); i++) if (parts[3]!='headerpartial' || i>0) rows[i].setOptions(options);
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
        List<SObject> stored = lead ? (List<SObject>)[SELECT OwnerId, LastName FROM Lead WHERE LastName = 'Reject' OR LastName = 'OuterReject' OR LastName LIKE 'Keep%'] : (List<SObject>)[SELECT OwnerId, Subject FROM Case WHERE Subject = 'Reject' OR Subject = 'OuterReject' OR Subject LIKE 'Keep%'];
        Integer assigned = 0;
        for (SObject row : stored) if (String.valueOf(row.get('OwnerId')).startsWith('00G')) assigned++;
        Map<String,String> owners = new Map<String,String>();
        for (SObject row : stored) owners.put(String.valueOf(row.get(lead ? 'LastName' : 'Subject')), String.valueOf(row.get('OwnerId')).startsWith('00G') ? 'queue' : 'user');
        observed.put('owners', owners);
        observed.put('stored', stored.size());
        observed.put('assigned', assigned);
        observed.put('events', events);
        observed.put('successes', successes);
        observed.put('errors', errors);
        return JSON.serialize(observed);
    }
}
