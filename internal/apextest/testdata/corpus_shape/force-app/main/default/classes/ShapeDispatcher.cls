public class ShapeDispatcher {
    public static void dispatch(System.TriggerOperation operation, List<Account> records) {
        ShapeDomain.Factory factory = new ShapeDomain.Builder();
        ShapeDomain domain = factory.construct(records);
        switch on operation {
            when BEFORE_INSERT { domain.beforeInsert(); }
            when else { }
        }
    }
}
