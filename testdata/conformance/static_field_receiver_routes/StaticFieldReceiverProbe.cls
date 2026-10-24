public class StaticFieldReceiverProbe {
    public static void observe() { System.assertEquals('parcel', ReceiverValues.Text.trim()); }
}
