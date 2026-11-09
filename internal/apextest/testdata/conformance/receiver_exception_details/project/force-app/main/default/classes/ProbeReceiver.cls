public class ProbeReceiver {
    public class Service { public String value() { return 'ready'; } }
    public Service item;
    public ProbeReceiver(Boolean initialized) { if (initialized) item = new Service(); }
    public String read() { return item.value(); }
    public static String observe(Boolean initialized) {
        try { return 'value:' + new ProbeReceiver(initialized).read(); }
        catch (NullPointerException e) { return 'caught:' + e.getTypeName() + '|' + e.getMessage() + '|' + e.getLineNumber(); }
    }
}
