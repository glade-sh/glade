public class PendingOperand67 {
    public Integer Seed = initialize();
    public Integer Value { get { return 1; } set { Seed = value; } }
    public PendingTarget67 Target { get { return new PendingTarget67(); } }
    public Integer Broken { get { throw new IllegalArgumentException('getter'); } }
    private Integer initialize() { return 1; }
}
