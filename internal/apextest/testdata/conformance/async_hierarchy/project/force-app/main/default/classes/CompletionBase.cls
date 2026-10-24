public virtual class CompletionBase implements Finalizer {
 public String accept(WorkerRoot.Queued worker) { worker.touch(); return 'accepted'; }
 public virtual String complete() { return 'base'; }
 public void execute(FinalizerContext context) { insert new Account(Name=complete()); }
}
