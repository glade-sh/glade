public class CustomDuplicateProbe {
    private class UnknownCaseException extends Exception {}
    private static String graph(Datacloud.DuplicateResult result, Map<Id,String> labels) {
        List<String> members = new List<String>();
        for (Datacloud.MatchResult groupResult : result.getMatchResults()) {
            for (Datacloud.MatchRecord member : groupResult.getMatchRecords()) {
                Id idValue = member.getRecord().Id;
                members.add(labels.containsKey(idValue) ? labels.get(idValue) : 'other');
            }
        }
        members.sort();
        return result.getDuplicateRule() + '/' + result.isAllowSave() + '[' + String.join(members, ',') + ']';
    }

    private static String find(SObject candidate, String targetRule, Map<Id,String> labels) {
        List<String> graphs = new List<String>();
        List<Datacloud.FindDuplicatesResult> results = Datacloud.FindDuplicates.findDuplicates(new List<SObject>{candidate});
        if (results.size() != 1 || !results[0].isSuccess()) return 'find-error';
        for (Datacloud.DuplicateResult result : results[0].getDuplicateResults()) {
            String value = graph(result, labels);
            if (targetRule == result.getDuplicateRule() || !value.endsWith('[]')) graphs.add(value);
        }
        graphs.sort();
        return graphs.isEmpty() ? 'no-rule' : String.join(graphs, ',');
    }

    private static String saveResult(Database.SaveResult saved, Map<Id,String> labels) {
        if (saved.isSuccess()) return 'saved';
        List<String> errors = new List<String>();
        for (Database.Error error : saved.getErrors()) {
            String value = String.valueOf(error.getStatusCode());
            if (error instanceof Database.DuplicateError) {
                value += '/' + graph(((Database.DuplicateError)error).getDuplicateResult(), labels);
            }
            errors.add(value);
        }
        errors.sort();
        return String.join(errors, ',');
    }

    private static SObject sample(String mode, Boolean person, String variant) {
        Boolean reference = variant == 'seed';
        Boolean candidate = variant == 'hit';
        if (person) {
            Id rt = Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('PersonAccount').getRecordTypeId();
            return new Account(RecordTypeId=rt, FirstName=reference ? 'Robert' : candidate ? 'Bob' : 'Zelda',
                LastName=reference || candidate ? 'Hartwell' : 'Quillstone');
        }
        return new Contact(Department=mode, LastName=reference ? 'Aster' : candidate ? 'Birch' : 'Quillstone',
            Phone=reference ? '4155551234' : candidate ? '4155551235' : '2028410150');
    }

    private static String upsertResult(Database.UpsertResult result, Map<Id,String> labels) {
        if (result.isSuccess()) return 'saved/' + result.isCreated();
        List<String> errors = new List<String>();
        for (Database.Error error : result.getErrors()) {
            String value = String.valueOf(error.getStatusCode());
            if (error instanceof Database.DuplicateError)
                value += '/' + graph(((Database.DuplicateError)error).getDuplicateResult(), labels);
            errors.add(value);
        }
        errors.sort();
        return String.join(errors, ',');
    }

    private static List<String> upsertEdits(String mode, Boolean person, Map<Id,String> labels, SObject decoy) {
        SObject fresh = sample(mode, person, 'hit');
        Database.UpsertResult addResult = Database.upsert(fresh, false);
        List<String> parts = new List<String>{'upsertInsert=' + upsertResult(addResult, labels)};
        if (addResult.isSuccess()) delete fresh;
        SObject edit = sample(mode, person, 'hit');
        edit.Id = decoy.Id;
        Database.UpsertResult editResult = Database.upsert(edit, false);
        parts.add('upsertUpdate=' + upsertResult(editResult, labels));
        return parts;
    }

    private static String unchangedEdit(String mode, Boolean person, Map<Id,String> labels) {
        SObject left = sample(mode, person, 'seed');
        SObject right = sample(mode, person, 'hit');
        if (person) {
            left.put('LastName','Cloudgarden');
            right.put('LastName','Cloudgarden');
        } else {
            left.put('Phone','3128821210');
            right.put('Phone','3128821211');
        }
        List<SObject> batch = new List<SObject>{left,right};
        List<Database.SaveResult> saves = Database.insert(batch, false);
        if (!saves[0].isSuccess() || !saves[1].isSuccess())
            return 'seeds/' + saveResult(saves[0], labels) + '/' + saveResult(saves[1], labels);
        labels.put(saves[0].getId(), 'left');
        labels.put(saves[1].getId(), 'right');
        SObject changed = person ? (SObject)new Account() : (SObject)new Contact();
        changed.Id = saves[1].getId();
        changed.put('Description','Updated unrelated field');
        return saveResult(Database.update(changed, false), labels);
    }

    public static String run(String key) {
        Boolean extended = key.endsWith('Upsert');
        String mode = extended ? key.substring(0,key.length()-6) : key;
        Map<String,String> rules = new Map<String,String>{
            'CustomBlock'=>'ProbePhoneBlock', 'CustomAlert'=>'ProbePhoneAlert',
            'CustomReport'=>'ProbePhoneReport', 'PersonCustom'=>'ProbePersonAlert'};
        if (!rules.containsKey(mode)) throw new UnknownCaseException('Unknown case');
        Boolean person = mode == 'PersonCustom';
        Savepoint point = Database.setSavepoint();
        try {
            SObject seed = sample(mode, person, 'seed');
            SObject decoy = sample(mode, person, 'miss');
            insert seed;
            insert decoy;
            Map<Id,String> labels = new Map<Id,String>{seed.Id=>'seed',decoy.Id=>'decoy'};
            if (extended) {
                List<String> observations = upsertEdits(mode, person, labels, decoy);
                observations.add('unrelatedUpdate=' + unchangedEdit(mode, person, labels));
                return String.join(observations,';');
            }
            SObject hit = sample(mode, person, 'hit');
            SObject miss = person
                ? (SObject)new Account(RecordTypeId=(Id)seed.get('RecordTypeId'),FirstName='Helen',LastName='Windrush')
                : (SObject)new Contact(Department=mode,LastName='Windrush',Phone='3036690151');
            List<String> parts = new List<String>{'hit=' + find(hit, rules.get(mode), labels),
                                                   'miss=' + find(miss, rules.get(mode), labels)};
            Database.DMLOptions options = new Database.DMLOptions();
            options.DuplicateRuleHeader.allowSave = true;
            for (Integer i=0; i<2; i++) {
                SObject added = sample(mode, person, 'hit');
                Database.SaveResult created = i==0 ? Database.insert(added, false) : Database.insert(added, options);
                parts.add((i==0 ? 'insert=' : 'insertOverride=') + saveResult(created, labels));
                if (created.isSuccess()) delete added;
            }
            for (Integer i=0; i<2; i++) {
                SObject edit = sample(mode, person, 'hit');
                edit.Id = decoy.Id;
                Database.SaveResult updated = i==0 ? Database.update(edit, false) : Database.update(edit, options);
                parts.add((i==0 ? 'update=' : 'updateOverride=') + saveResult(updated, labels));
                if (updated.isSuccess()) update decoy;
            }
            return String.join(parts, ';');
        } finally {
            Database.rollback(point);
            Database.releaseSavepoint(point);
        }
    }
}
