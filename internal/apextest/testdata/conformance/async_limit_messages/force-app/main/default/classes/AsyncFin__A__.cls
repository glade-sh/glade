public class AsyncFin__A__ implements Finalizer {
    String mode;
    public AsyncFin__A__(String mode) { this.mode = mode; }
    public void execute(FinalizerContext fc) {
        if (mode == 'fsoql150') { for (Integer i = 0; i < 150; i++) { List<Account> r = [SELECT Id FROM Account WHERE Id = null]; } }
        AsyncProbe__A__.rec(mode + '_fin', AsyncProbe__A__.info());
    }
}
