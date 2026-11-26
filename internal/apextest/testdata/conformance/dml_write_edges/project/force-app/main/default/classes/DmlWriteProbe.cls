public class DmlWriteProbe {
    public static Integer insertHits = 0;
    private static String saveResult(Database.SaveResult result) {
        return result.isSuccess() ? 'ok' : String.valueOf(result.getErrors()[0].getStatusCode());
    }

    public static Map<String, String> lookupRows() {
        Map<String, String> rows = new Map<String, String>();
        Lantern__c parent = new Lantern__c(Name = 'lit');
        Lantern__c removed = new Lantern__c(Name = 'removed');
        insert new List<Lantern__c>{parent, removed};
        delete removed;
        Id missing = (Id)(Lantern__c.SObjectType.getDescribe().getKeyPrefix() + '000000ZZZZZZ');
        Id wrong = (Id)(Account.SObjectType.getDescribe().getKeyPrefix() + '000000ZZZZZZ');
        List<Pebble__c> children = new List<Pebble__c>{
            new Pebble__c(Name = 'missing', Lantern__c = missing),
            new Pebble__c(Name = 'live', Lantern__c = parent.Id),
            new Pebble__c(Name = 'wrong', Lantern__c = wrong),
            new Pebble__c(Name = 'removed', Lantern__c = removed.Id),
            new Pebble__c(Name = 'empty')
        };
        List<Database.SaveResult> results = Database.insert(children, false);
        for (Integer i = 0; i < children.size(); i++) {
            rows.put('lookup_' + children[i].Name, saveResult(results[i]));
        }
        if (!results[0].isSuccess()) {
            Database.Error issue = results[0].getErrors()[0];
            rows.put('lookup_missing_detail', String.join(issue.getFields(), ',') + '|' + maskId(issue.getMessage(), missing));
        } else {
            rows.put('lookup_missing_detail', 'ok');
        }
        Database.Error wrongIssue = results[2].getErrors()[0];
        rows.put('lookup_wrong_detail', String.join(wrongIssue.getFields(), ',') + '|' + maskId(wrongIssue.getMessage(), wrong));
        try {
            insert new Pebble__c(Name = 'statement', Lantern__c = missing);
            rows.put('lookup_statement', 'ok');
        } catch (DmlException e) {
            rows.put('lookup_statement', String.valueOf(e.getDmlStatusCode(0)));
        }
        rows.put('lookup_persisted', String.valueOf([SELECT count() FROM Pebble__c WHERE Name IN ('missing', 'statement')]));
        return rows;
    }

    private static String maskId(String message, Id idValue) {
        String text = String.valueOf(idValue);
        return message.replace(text, '<reference>').replace(text.left(15), '<reference>');
    }

    private static String upsertResult(Database.UpsertResult result) {
        return (result.isCreated() ? 'created' : 'not-created') + '|' +
            (result.getId() == null ? 'no-id' : 'id') + '|' +
            (result.isSuccess() ? 'ok' : String.valueOf(result.getErrors()[0].getStatusCode()) + '|' +
                String.join(result.getErrors()[0].getFields(), ','));
    }

    public static Map<String, String> upsertRows() {
        Map<String, String> rows = new Map<String, String>();
        List<Database.UpsertResult> results = Database.upsert(new List<Pebble__c>{
            new Pebble__c(Name = 'long key', Token__c = 'excess')
        }, Pebble__c.Fields.Token__c, false);
        rows.put('upsert_long_key', upsertResult(results[0]));
        rows.put('upsert_long_key_message', results[0].getErrors()[0].getMessage());
        rows.put('upsert_long_key_trigger', String.valueOf(insertHits));

        Lantern__c removed = new Lantern__c(Name = 'removed');
        insert removed;
        delete removed;
        results = Database.upsert(new List<Pebble__c>{
            new Pebble__c(Name = 'long value', Token__c = 'short', Short__c = 'too long here'),
            new Pebble__c(Name = 'removed parent', Token__c = 'dead', Lantern__c = removed.Id)
        }, Pebble__c.Fields.Token__c, false);
        rows.put('upsert_long_value', upsertResult(results[0]));
        rows.put('upsert_removed_parent', upsertResult(results[1]));

        results = Database.upsert(new List<Pebble__c>{
            new Pebble__c(Name = 'long key mixed', Token__c = 'longer'),
            new Pebble__c(Name = 'new key mixed', Token__c = 'mixed')
        }, Pebble__c.Fields.Token__c, false);
        rows.put('upsert_long_key_mixed', upsertResult(results[0]));
        rows.put('upsert_new_key_mixed', upsertResult(results[1]));
        rows.put('upsert_persisted', String.valueOf([SELECT count() FROM Pebble__c WHERE Name IN ('long key', 'long key mixed')]));
        return rows;
    }

    public static String frame(String value) {
        return value == null ? 'null' : '[' + value.replace('\t', '#tab#').replace('\n', '#lf#') + ']';
    }

    private static Pebble__c textRow(String name, String value) {
        return new Pebble__c(Name = name, Short__c = value, Note__c = value, LongNote__c = value);
    }

    private static String textResult(Pebble__c input) {
        Pebble__c stored = [SELECT Name, Short__c, Note__c, LongNote__c, Observed__c FROM Pebble__c WHERE Id = :input.Id];
        return frame(input.Name) + '|' + frame(input.Short__c) + '|' + frame(input.Observed__c) + '|' +
            frame(stored.Name) + '|' + frame(stored.Short__c) + '|' +
            frame(stored.Note__c) + '|' + frame(stored.LongNote__c) + '|' + stored.Observed__c;
    }

    public static Map<String, String> textRows() {
        Map<String, String> rows = new Map<String, String>();
        Pebble__c spaced = textRow('  spaced  ', '  glow  ');
        Pebble__c blank = textRow('blank', '   ');
        Pebble__c tabbed = textRow('tabbed', '\tglow\t');
        insert new List<Pebble__c>{spaced, blank, tabbed};
        rows.put('text_spaced', textResult(spaced));
        rows.put('text_blank', textResult(blank));
        rows.put('text_tabbed', textResult(tabbed));
        Pebble__c updated = textRow('  updated  ', '  dim  ');
        updated.Id = spaced.Id;
        update updated;
        rows.put('text_update', textResult(updated));

        Pebble__c inserted = textRow('  upsert  ', '  dusk  ');
        inserted.Token__c = 'dusk';
        Database.upsert(new List<Pebble__c>{inserted}, Pebble__c.Fields.Token__c, false);
        rows.put('text_upsert', textResult(inserted));
        return rows;
    }
}
