public class OwnedBroadChild extends OwnedBroad {
    public Schema.Group inheritedGroup() { return groupField; }
    public Request siblingRequest() { return new Request(); }
    public Message siblingMessage() { return new ChildMessage(); }
}
