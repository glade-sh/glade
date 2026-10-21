public virtual class CompletionSink implements Finalizer {
    public Integer count = 0;
    public void append(WorkUnit work) { count++; insert new Account(Name='callback completed'); }
    public virtual void execute(FinalizerContext context) { handleSuccess(); }
    protected virtual void handleSuccess() {}
}
