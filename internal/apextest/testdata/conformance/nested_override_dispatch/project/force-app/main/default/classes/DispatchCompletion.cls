public virtual class DispatchCompletion implements Finalizer {
 public virtual String complete() { return 'base'; }
 public void execute(FinalizerContext context) { insert new Account(Name=complete()); }
}
