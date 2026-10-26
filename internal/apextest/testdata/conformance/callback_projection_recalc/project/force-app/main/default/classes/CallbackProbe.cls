public class CallbackProbe {
    public static String observe() {
        CallbackBase finalizer = new SpecificCallback();
        CallbackProcessor worker = new AsyncCallbackProcessor();
        return finalizer.addCaboose(worker);
    }
}
