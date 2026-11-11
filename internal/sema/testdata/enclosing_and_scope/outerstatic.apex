public class Probe { public abstract class Inner {} public static Boolean ok() { return true; } public static Boolean run() { return Probe.Inner.ok(); } }
