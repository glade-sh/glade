public class OwnedEntryHook implements EntryHooks.VoidHook, EntryHooks.CountHook {
    public static Integer calls = 0;
    public static String seen = '';
    public void apply(List<OwnedEntry__c> rows) {
        calls++;
        seen = rows == null ? 'null' : String.valueOf(rows.size());
    }
    public Integer count(List<OwnedEntry__c> rows) {
        return rows == null ? -1 : rows.size();
    }
}
