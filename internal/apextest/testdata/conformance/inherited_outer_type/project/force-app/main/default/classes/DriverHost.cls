public class DriverHost {
    public class Request { public Integer count = 3; }
    public abstract class BaseDriver { public abstract String run(Request value); }
    public interface Driver { String run(Request value); }
}
