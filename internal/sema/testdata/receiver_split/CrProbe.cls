public class CrProbe {
    public static String SF = 'sf';
    public String name = 'n';
    public Kid child = new Kid();
    public enum Color { RED }
    public class Kid {
        public String name = 'kid';
        public String call() { return 'call'; }
    }
    public static String value() { return 'ok'; }
    public String instanceValue() { return 'ok'; }
}
