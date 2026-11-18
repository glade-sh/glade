public class ResidualThrow {
 public class BoomException extends Exception {}
 @InvocableMethod(label='Residual throw')
 public static void run(List<Boolean> items) { throw new BoomException('boom'); }
}
