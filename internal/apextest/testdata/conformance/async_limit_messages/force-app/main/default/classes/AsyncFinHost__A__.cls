public class AsyncFinHost__A__ implements Queueable {
    String mode;
    public AsyncFinHost__A__(String mode) { this.mode = mode; }
    public void execute(QueueableContext ctx) { System.attachFinalizer(new AsyncFin__A__(mode)); }
}
