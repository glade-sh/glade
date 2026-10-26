public class ShapePublisher {
    // The unrelated inner EventBus must not hide the implicit System receiver.
    // Native: inner_type_shapes/org.tsv:50-53; inner_type_scope/org.tsv:2-3.
    public static Integer publishEmpty() {
        return EventBus.publish(new List<SObject>()).size();
    }
}
