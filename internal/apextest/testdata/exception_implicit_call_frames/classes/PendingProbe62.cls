public class PendingProbe62 {
    public static void run(String mode) {
        PendingOperand62 operand = new PendingOperand62();
        PendingTarget62 target = new PendingTarget62();
        if (mode == 'literal') {
            target
                .fail(1);
        }
        if (mode == 'getter') {
            target
                .fail(operand.Value);
        }
        if (mode == 'setter') {
            target
                .fail(operand.Value++);
        }
        if (mode == 'receiver') {
            operand.Target
                .fail(1);
        }
        if (mode == 'initializer') {
            target
                .fail(PendingLazy62.Value);
        }
        if (mode == 'constructor') {
            target
                .fail(new PendingOperand62().Value);
        }
        if (mode == 'broken') {
            target
                .fail(operand.Broken);
        }
    }
    public static String capture(String mode) {
        try { run(mode); } catch (Exception e) { return e.getStackTraceString(); }
        return 'no exception';
    }
}
