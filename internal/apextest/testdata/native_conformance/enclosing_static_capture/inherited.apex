public virtual class Parent { public static Boolean ok() { return true; } }
public class Probe extends Parent { public class Adapter {} public static Boolean run() { return Probe.Adapter.ok(); } }
