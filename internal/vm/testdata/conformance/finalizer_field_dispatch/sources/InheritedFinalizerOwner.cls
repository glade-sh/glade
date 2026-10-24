public virtual class InheritedFinalizerOwner {
    protected InheritedFinalizerCollector finalizer=new InheritedFinalizerCollector();
    public virtual class Stage extends InheritedFinalizerOwner implements System.Queueable {
        public void execute(System.QueueableContext context) {}
    }
}
