public class BulkReferenceProbe {
    public class Bucket {
        public List<Account> rows = new List<Account>();
        public Integer countRows() { return rows.size(); }
    }
    public class Ledger {
        public List<Bucket> buckets = new List<Bucket>();
        public Ledger(List<Account> rows) {
            for (Account row : rows) {
                Bucket bucket = new Bucket();
                bucket.rows.add(row);
                buckets.add(bucket);
            }
        }
        public Integer countRows() {
            Integer total = 0;
            for (Bucket bucket : buckets) { total += bucket.countRows(); }
            return total;
        }
    }
    public static void apply(List<Account> rows) {
        Ledger ledger = new Ledger(rows);
        System.assertEquals(rows.size(), ledger.countRows());
        for (Account row : rows) { row.Description = 'processed'; }
    }
    public static Map<String,Object> observe() {
        List<Account> rows = new List<Account>();
        for (Integer i = 0; i < 200; i++) { rows.add(new Account(Name='Synthetic ' + i)); }
        insert rows;
        for (Account row : rows) { row.Name += ' updated'; }
        update rows;
        Integer processed = 0;
        for (Account row : [SELECT Id, Description FROM Account WHERE Id IN :rows]) {
            if (row.Description == 'processed') { processed++; }
        }
        return new Map<String,Object>{'inserted'=>rows.size(), 'processed'=>processed};
    }
}
