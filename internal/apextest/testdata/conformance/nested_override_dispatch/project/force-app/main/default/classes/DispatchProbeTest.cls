@IsTest private class DispatchProbeTest {
 @IsTest static void observe() {
  Map<String,Object> value=DispatchProbe.observe();
  Test.startTest(); new DispatchLeaf().prepare().queue(); Test.stopTest();
  value.put('async',[SELECT Name FROM Account LIMIT 1].Name);
  System.assert(false,'OBS|'+JSON.serialize(value));
 }
}
