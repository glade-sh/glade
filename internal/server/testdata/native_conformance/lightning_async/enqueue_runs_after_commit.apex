Id jobId = System.enqueueJob(new GladeQueueProbe('ok<API>'));
System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'glade-queue-probe-ok<API>']);
