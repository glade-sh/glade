public class LazyCacheState {
 public static Integer builds=0;
 private static Map<String,Integer> entries {
  get {
   if(entries==null) entries=build();
   return entries;
  }
  set;
 }
 private static Map<String,Integer> build() {
  builds++;
  GuardedCacheRelay.observe();
  return new Map<String,Integer>{'sample'=>7};
 }
 public static Boolean hasEntry() {
  return entries.containsKey('sample');
 }
}
