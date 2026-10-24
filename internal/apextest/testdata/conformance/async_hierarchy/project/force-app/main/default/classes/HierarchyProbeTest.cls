@IsTest private class HierarchyProbeTest { @IsTest static void observe() { Map<String,Object> value=HierarchyProbe.observe();
Test.startTest(); System.enqueueJob(new WorkerOuter.Variant()); Test.stopTest();
value.put('asyncResult',[SELECT Name FROM Account LIMIT 1].Name);
System.assert(false,'OBS|'+JSON.serialize(value)); } }
