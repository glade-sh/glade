public class TransactionDml {
    public static void setupRow(String label) {
        insert new PermissionSet(Name='Boundary'+label, Label='Boundary '+label);
    }
    public static void dataRow(String label) {
        Database.insert(new Account(Name='Boundary '+label));
    }
    public static void rejectOpposite(Boolean setup, String label) {
        Boolean rejected=false;
        try {
            if (setup) dataRow(label); else setupRow(label);
        } catch (DmlException e) {
            rejected=String.valueOf(e.getDmlType(0))=='MIXED_DML_OPERATION';
        }
        System.assert(rejected, 'same transaction must reject opposite DML: '+label);
    }
    public static void work(Boolean setup, String label) {
        if (setup) setupRow(label); else dataRow(label);
        rejectOpposite(setup, label+'Rejected');
    }
    @future public static void later(Boolean setup, String label) { work(setup,label); }
}
