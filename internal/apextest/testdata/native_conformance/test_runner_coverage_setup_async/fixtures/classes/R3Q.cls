public class R3Q implements Queueable {
    public void execute(QueueableContext ctx) {
        insert new Account(Name = 'R3-Q');
    }
}
