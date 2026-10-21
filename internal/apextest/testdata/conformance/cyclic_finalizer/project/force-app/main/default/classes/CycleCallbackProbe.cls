public virtual class CycleCallbackProbe implements Finalizer {
    public virtual void execute(FinalizerContext context) {
    }

    public void marker() {
        insert new Account(Name = 'Cyclic observer');
    }
}
