public class ReceiverControl {
    public ReceiverDependency instance = new ReceiverDependency();
    public static ReceiverDependency shared = new ReceiverDependency();
    public String instanceValue() { return instance.value(); }
    public static String staticValue() { return shared.value(); }
}
