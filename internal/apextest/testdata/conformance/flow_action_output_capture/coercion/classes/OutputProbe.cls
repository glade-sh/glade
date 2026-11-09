public class OutputProbe {
 public class Req { @InvocableVariable public String label; }
 public class Res {
  @InvocableVariable public String text; @InvocableVariable public String explicitNull;
  @InvocableVariable public String untouched; @InvocableVariable public Decimal amount;
  @InvocableVariable public Date day; @InvocableVariable public List<String> items;
 }
 @InvocableMethod public static List<Res> run(List<Req> inputs) {
  List<Res> results = new List<Res>();
  for (Req input : inputs) { Res r = new Res(); r.text = input.label; r.explicitNull = null; r.amount = input.label == 'negative' ? -7.25 : input.label == 'zero' ? 0 : 12.5; r.day = Date.newInstance(2026, 11, 3); r.items = new List<String>{input.label, 'two'}; results.add(r); }
  return results;
 }
}
