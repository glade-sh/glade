public virtual class CycleWorkerBaseProbe {
    protected CycleCallbackProbe callback;

    public void finish() {
        this.callback.marker();
    }
}
