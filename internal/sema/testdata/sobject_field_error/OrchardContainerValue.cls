public class OrchardContainerValue {
    public class Holder { public Decimal amount; }
    public static void run() {
        Holder holder = new Holder(); holder.amount.addError('invalid value');
    }
}
