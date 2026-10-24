public class InheritedFinalizerCollector implements System.Finalizer {
    public Integer total=0;
    public void append(InheritedFinalizerOwner value) { total++; }
    public void execute(System.FinalizerContext context) {}
}
