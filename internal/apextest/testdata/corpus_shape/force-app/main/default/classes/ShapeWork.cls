public class ShapeWork {
    private List<SObject> pending = new List<SObject>();
    public void registerNew(SObject record) { pending.add(record); }
    public void commitWork() {
        Savepoint boundary = Database.setSavepoint();
        try { insert pending; }
        catch (Exception problem) { Database.rollback(boundary); throw problem; }
    }
}
