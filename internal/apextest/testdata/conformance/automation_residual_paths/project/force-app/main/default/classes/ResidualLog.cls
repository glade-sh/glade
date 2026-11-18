public class ResidualLog {
 public static List<String> rows = new List<String>();
 public static void add(EventBus.ChangeEventHeader h, String extra) {
  List<String> f = h.changedFields == null ? new List<String>() : h.changedFields.clone(); f.sort();
  rows.add(h.entityName + ':' + h.changeType + ':' + String.join(f, ',') + ':' + extra);
 }
}
