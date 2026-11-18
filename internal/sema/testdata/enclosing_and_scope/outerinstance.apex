public class Probe { public abstract class Inner {} public Boolean onlyInstance() { return true; } public static Boolean run() { return Probe.Inner.onlyInstance(); } }
