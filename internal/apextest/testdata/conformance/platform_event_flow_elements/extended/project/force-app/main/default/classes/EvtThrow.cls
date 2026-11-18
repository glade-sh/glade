public class EvtThrow {
 @InvocableMethod public static void boom(List<String> keys) { throw new IllegalArgumentException('boom ' + keys[0]); }
}
