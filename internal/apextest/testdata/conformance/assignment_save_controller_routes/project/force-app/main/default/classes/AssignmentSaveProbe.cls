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
        SObject row = lead ? (SObject)new Lead(LastName='Reject', Company='Route') : (SObject)new Case(Subject='Reject', Origin='Web', Status='New');
        if (updating) insert row;
        events.clear();
        rejectAt = parts[3];
        Database.DMLOptions options = new Database.DMLOptions();
        options.assignmentRuleHeader.useDefaultRule = true;
        options.optAllOrNone = true;
        row.setOptions(options);
        Test.setCurrentPage(new PageReference('/apex/AssignmentSaveRoute'));
        ApexPages.StandardController controller = new ApexPages.StandardController(row);
        Map<String,Object> observed = new Map<String,Object>();
        try {
            PageReference page = controller.save();
            observed.put('pageNull', page == null);
            observed.put('exception', 'none');
        } catch (DmlException e) {
            observed.put('exception', 'DmlException');
            observed.put('exceptionCode', String.valueOf(e.getDmlStatusCode(0)));
            observed.put('exceptionIndex', e.getDmlIndex(0));
        }
        List<SObject> stored = lead ? (List<SObject>)[SELECT OwnerId FROM Lead WHERE LastName='Reject'] : (List<SObject>)[SELECT OwnerId FROM Case WHERE Subject='Reject'];
        Integer assigned = 0;
        for (SObject saved : stored) if (String.valueOf(saved.get('OwnerId')).startsWith('00G')) assigned++;
        if (parts[3] == 'followup') {
            observed.put('controllerAssigned', assigned);
            row.Id = controller.getId();
            Database.SaveResult followup = Database.update(row, false);
            observed.put('followupSuccess', followup.isSuccess());
            stored = lead ? (List<SObject>)[SELECT OwnerId FROM Lead WHERE LastName='Reject'] : (List<SObject>)[SELECT OwnerId FROM Case WHERE Subject='Reject'];
            assigned = 0;
            for (SObject saved : stored) if (String.valueOf(saved.get('OwnerId')).startsWith('00G')) assigned++;
        }
        observed.put('stored', stored.size());
        observed.put('assigned', assigned);
        observed.put('events', events);
        observed.put('pageMessages', ApexPages.getMessages().size());
        return JSON.serialize(observed);
    }
}
