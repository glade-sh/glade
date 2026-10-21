public class AliasLoopProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> observed = new Map<String,Object>();
        for (String mode : new List<String>{'assignment', 'put'}) {
            List<Account> rows = new List<Account>();
            for (Integer i = 0; i < 3; i++) {
                rows.add(new Account(Name='Synthetic ' + i));
            }
            Account first = rows[0];
            List<Account> duplicates = new List<Account>{first, first};
            for (Account row : rows) {
                if (mode == 'assignment') { row.Description = 'processed'; }
                else { row.put('Description', 'processed'); }
            }
            Integer processed = 0;
            for (Account row : rows) {
                if (row.Description == 'processed') { processed++; }
            }
            observed.put(mode + 'Count', processed);
            observed.put(mode + 'Direct', first.Description);
            observed.put(mode + 'Duplicate0', duplicates[0].Description);
            observed.put(mode + 'Duplicate1', duplicates[1].Description);
        }
        return observed;
    }
}
