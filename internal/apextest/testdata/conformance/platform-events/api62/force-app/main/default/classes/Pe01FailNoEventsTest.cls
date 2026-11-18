@IsTest private class Pe01FailNoEventsTest {
 @IsTest static void observe() {
  Map<String,Object> p=new Map<String,Object>{'before'=>PeProbe.names().size()};
  try { Test.getEventBus().fail(); p.put('fail1','ok'); } catch(Exception e) { p.put('fail1',PeProbe.error(e)); }
  try { Test.getEventBus().deliver(); p.put('deliver','ok'); } catch(Exception e) { p.put('deliver',PeProbe.error(e)); }
  try { Test.getEventBus().fail(); p.put('fail2','ok'); } catch(Exception e) { p.put('fail2',PeProbe.error(e)); }
  p.put('after',PeProbe.names().size());
  System.assert(false,'OBS|'+JSON.serialize(p));
 }
}
