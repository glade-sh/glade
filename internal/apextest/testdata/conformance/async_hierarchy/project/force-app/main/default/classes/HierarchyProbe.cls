public class HierarchyProbe {
 public static void queue() { System.enqueueJob(new WorkerOuter.Variant()); }
 public static Map<String,Object> observe() {
  return new Map<String,Object>{'callback' => new WorkerOuter().callback(), 'outer' => new WorkerOuter().run(), 'variant' => new WorkerOuter.Variant().run()};
 }
}
