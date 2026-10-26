public virtual class ShapeDomain {
    protected List<Account> records;
    public ShapeDomain(List<Account> records) { this.records = records; }
    public virtual void beforeInsert() {
        for (Account record : records) {
            if (record.Description == null) record.Description = 'canopy';
        }
    }
    public interface Factory { ShapeDomain construct(List<Account> records); }
    public class Builder implements Factory {
        public ShapeDomain construct(List<Account> records) { return new ShapeDomain(records); }
    }
}
