public class PendingProbe67 {
    public static void run(String mode) {
        PendingOperand67 operand = new PendingOperand67();
        PendingTarget67 target = new PendingTarget67();
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
                .fail(PendingLazy67.Value);
        }
        if (mode == 'constructor') {
            target
                .fail(new PendingOperand67().Value);
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
