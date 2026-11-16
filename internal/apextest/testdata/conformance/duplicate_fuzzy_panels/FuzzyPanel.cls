public class FuzzyPanel {
    private static final String DIGITS = '0123456789abcdefghijklmnopqrstuvwxyz';
    private class PanelException extends Exception {}

    private static SObject record(String objectName, List<String> fields, String value) {
        SObject row = Schema.getGlobalDescribe().get(objectName).newSObject();
        List<String> parts = value.split('\\|', -1);
        if (parts.size() != fields.size()) throw new PanelException('field count');
        for (Integer i = 0; i < fields.size(); i++) {
            if (parts[i] != '') row.put(fields[i], parts[i]);
        }
        String required = objectName == 'Account' ? 'Name' : 'LastName';
        if (row.get(required) == null) row.put(required, 'Blankrow');
        return row;
    }

    // Inserts one record per value, then asks Datacloud.FindDuplicates about an
    // unsaved copy of every value. Each match of the named rule is reported as
    // candidate index, existing index and, when it is not 100, the confidence.
    public static String run(String objectName, String rule, String fieldList, List<String> values) {
        if (values.size() > 36) throw new PanelException('too many values');
        List<String> fields = fieldList.split('\\+');
        List<SObject> seeds = new List<SObject>();
        List<SObject> candidates = new List<SObject>();
        for (String value : values) {
            seeds.add(record(objectName, fields, value));
            candidates.add(record(objectName, fields, value));
        }
        Database.DMLOptions options = new Database.DMLOptions();
        options.DuplicateRuleHeader.allowSave = true;
        List<Database.SaveResult> saved = Database.insert(seeds, options);
        Map<Id, Integer> index = new Map<Id, Integer>();
        for (Integer i = 0; i < saved.size(); i++) {
            if (!saved[i].isSuccess()) throw new PanelException('insert ' + i + ' ' + saved[i].getErrors()[0].getStatusCode());
            index.put(saved[i].getId(), i);
        }
        List<Datacloud.FindDuplicatesResult> results = Datacloud.FindDuplicates.findDuplicates(candidates);
        if (results.size() != candidates.size()) throw new PanelException('result count');
        List<String> out = new List<String>();
        for (Integer i = 0; i < results.size(); i++) {
            if (!results[i].isSuccess()) throw new PanelException('find ' + i);
            for (Datacloud.DuplicateResult result : results[i].getDuplicateResults()) {
                if (result.getDuplicateRule() != rule) continue;
                for (Datacloud.MatchResult match : result.getMatchResults()) {
                    for (Datacloud.MatchRecord member : match.getMatchRecords()) {
                        Integer j = index.get(member.getRecord().Id);
                        String token = DIGITS.substring(i, i + 1) + (j == null ? '?' : DIGITS.substring(j, j + 1));
                        Double confidence = member.getMatchConfidence();
                        if (confidence != 100) token += String.valueOf(confidence);
                        out.add(token);
                    }
                }
            }
        }
        out.sort();
        String observation = values.size() + ':' + String.join(out, ',');
        if (observation.length() > 3900) throw new PanelException('observation too long');
        return observation;
    }
}
