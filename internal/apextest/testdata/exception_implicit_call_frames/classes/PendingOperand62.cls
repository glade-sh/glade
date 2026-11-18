public class PendingOperand62 {
    public Integer Seed = initialize();
    public Integer Value { get { return 1; } set { Seed = value; } }
    public PendingTarget62 Target { get { return new PendingTarget62(); } }
    public Integer Broken { get { throw new IllegalArgumentException('getter'); } }
    private Integer initialize() { return 1; }
}
