public class DispatchLeaf extends DispatchOuter {
 public DispatchLeaf() {super();}
 protected override String beginWork() { String ignored=super.beginWork();return 'leaf'; }
}
