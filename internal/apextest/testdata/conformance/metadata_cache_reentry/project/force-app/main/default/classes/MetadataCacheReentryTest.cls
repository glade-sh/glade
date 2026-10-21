@IsTest private class MetadataCacheReentryTest {
 @IsTest static void reentrantMap() {
  Map<String,Object> result=new Map<String,Object>();
  try {
   result.put('read',LazyCacheState.hasEntry());
   result.put('again',LazyCacheState.hasEntry());
  } catch(Exception issue) { result.put('error',issue.getTypeName()+'|'+issue.getMessage()); }
  result.put('builds',LazyCacheState.builds);
  result.put('calls',GuardedCacheRelay.calls);
  result.put('nested',GuardedCacheRelay.nestedRead);
  result.put('active',GuardedCacheRelay.active);
  System.assert(false,'OBSERVED|reentrantMap|'+JSON.serialize(result));
 }
 @IsTest static void directBacking() {
  GuardedCacheRelay.skip=true;
  Map<String,Object> result=new Map<String,Object>();
  try {
   result.put('read',LazyCacheState.hasEntry());
   result.put('again',LazyCacheState.hasEntry());
  } catch(Exception issue) { result.put('error',issue.getTypeName()+'|'+issue.getMessage()); }
  result.put('builds',LazyCacheState.builds);
  result.put('calls',GuardedCacheRelay.calls);
  result.put('active',GuardedCacheRelay.active);
  System.assert(false,'OBSERVED|directBacking|'+JSON.serialize(result));
 }
}
