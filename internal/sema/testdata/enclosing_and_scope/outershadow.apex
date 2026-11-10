public class Probe { public class Inner { public static Integer value() { return 2; } } public static Integer value() { return 1; } public static Integer run() { return Probe.Inner.value(); } }
