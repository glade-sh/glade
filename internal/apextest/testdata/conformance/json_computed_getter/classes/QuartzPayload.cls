public class QuartzPayload {
    public Integer seed;
    public Integer autoRead { get; }
    public Integer privateWrite { get; private set; }
    public Integer computed { get { return seed == null ? -1 : seed + 1; } }
    public Integer fallback { get { return fallback == null ? 7 : fallback; } }
    public String writable {
        get { return stored; }
        set { stored = 'set ' + value; }
    }
    public Integer readComputed() { return computed; }
    private String stored;
}
