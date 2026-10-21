public class InlineCursorReject {
    public static Integer count(List<SObject> rows) { return rows.size(); }
    public static Integer count(Iterator<Object> cursor) { return cursor.hasNext() ? 1 : 0; }
    public static Integer run() {
        return count(new List<Account>{new Account(Name='Inline')}.iterator());
    }
}
