public class Probe { public abstract class Adapter {} public Boolean onlyInstance() { return true; } public static Boolean run() { return Probe.Adapter.onlyInstance(); } }
