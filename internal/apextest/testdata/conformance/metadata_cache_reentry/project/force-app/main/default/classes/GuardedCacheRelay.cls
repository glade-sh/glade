public class GuardedCacheRelay {
 public static Boolean active=false;
 public static Boolean skip=false;
 public static Boolean nestedRead;
 public static Integer calls=0;
 public static void observe() {
  if(active || skip) return;
  active=true;
  try {
   calls++;
   nestedRead=LazyCacheState.hasEntry();
  } finally { active=false; }
 }
}
