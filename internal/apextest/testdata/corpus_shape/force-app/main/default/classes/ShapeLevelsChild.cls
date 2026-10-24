public class ShapeLevelsChild extends ShapeLevels {
    // Compile-only: inherited inner enum statics through a subclass.
    public static void childRoutes() {
        List<Status> statuses = Status.values();
        Status closed = Status.valueOf('CLOSED');
        Message note = Message.valueOf('NOTE');
    }
}
