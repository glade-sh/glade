public class SnQueue implements Queueable {
    public static List<String> rows = new List<String>();
    private String label;
    public SnQueue(String label) { this.label = label; }
    public void execute(QueueableContext ctx) {
        try {
            Integer d = AsyncInfo.getMinimumQueueableDelayInMinutes();
            String row = 'SNROW async.' + label + ' isnull=' + (d == null) + ' valueOf=' + String.valueOf(d) + ' pick=' + SnNull.g(d) + ' direct=' + SnNull.g(AsyncInfo.getMinimumQueueableDelayInMinutes());
            rows.add(row);
            System.debug(row);
        } catch (Exception e) {
            String row = 'SNROW async.' + label + ' EXC ' + e.getTypeName() + ': ' + e.getMessage();
            rows.add(row);
            System.debug(row);
        }
    }
}
