public class DispatchProbe {
 public static Map<String,Object> observe() {
  return new Map<String,Object>{'aliased'=>new DispatchLeaf().finishAliased(), 'outer'=>new DispatchOuter().finishDirect(), 'leaf'=>new DispatchLeaf().finishDirect()};
 }
}
