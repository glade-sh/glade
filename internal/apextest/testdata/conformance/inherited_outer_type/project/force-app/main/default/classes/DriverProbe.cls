public class DriverProbe {
    public static String direct() { return new NamedDriver().createRequest(); }
    public static String inherited() { return new LaterDriver().createRequest(); }
    public static String qualified() { DriverHost.BaseDriver driver = new NamedDriver(); return driver.run(new DriverHost.Request()); }
}
