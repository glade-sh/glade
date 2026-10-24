public class InheritedFinalizerWorker extends InheritedFinalizerOwner.Stage {
    public Integer exercise() {
        this.finalizer.append(this);
        finalizer.append(new InheritedFinalizerOwner());
        InheritedFinalizerCollector local=finalizer;
        local.append(this);
        return this.finalizer.total;
    }
}
