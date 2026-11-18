public class Probe { public abstract class Adapter {} public static Boolean ok() { return true; } public static Boolean run() { return Probe.Adapter.ok(); } }
