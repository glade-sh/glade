public class FlowIntegerProbe {
 public class Input { @InvocableVariable public Integer amount; }
 public static Object seen;
 @InvocableMethod public static void observe(List<Input> inputs) {
  Integer amount = (Integer)inputs[0].amount;
  Object boxed = amount;
  seen = new Map<String,Object>{'value'=>amount, 'integer'=>boxed instanceof Integer};
 }
}
