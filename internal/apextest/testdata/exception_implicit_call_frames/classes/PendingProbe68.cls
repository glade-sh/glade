public class PendingProbe68 {
    public static void run(String mode) {
        PendingOperand68 operand = new PendingOperand68();
        PendingTarget68 target = new PendingTarget68();
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
                .fail(PendingLazy68.Value);
        }
        if (mode == 'constructor') {
            target
                .fail(new PendingOperand68().Value);
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
