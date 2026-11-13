public class PendingOperand68 {
    public Integer Seed = initialize();
    public Integer Value { get { return 1; } set { Seed = value; } }
    public PendingTarget68 Target { get { return new PendingTarget68(); } }
    public Integer Broken { get { throw new IllegalArgumentException('getter'); } }
    private Integer initialize() { return 1; }
}
