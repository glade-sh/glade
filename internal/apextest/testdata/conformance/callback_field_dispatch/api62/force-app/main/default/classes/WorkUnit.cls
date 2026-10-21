public virtual class WorkUnit {
    protected CompletionSink callback;
    public WorkUnit() {}
    public void setCallback(CompletionSink value) { this.callback = value; }
    public void finish() { this.callback.append(this); }
}
