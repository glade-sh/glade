public class CursorSink {
    public static Integer count(List<SObject> rows) { return rows.size(); }
    public static Integer count(Iterator<Object> cursor) {
        Integer size = 0;
        while (cursor.hasNext()) { cursor.next(); size++; }
        return size;
    }
    public Integer accept(List<SObject> rows) { return rows.size(); }
    public Integer accept(Iterator<Object> cursor) { return count(cursor); }
    public static Integer run() {
        Iterator<Account> cursor = new List<Account>{new Account(Name='Seed')}.iterator();
        return count(cursor);
    }
}
