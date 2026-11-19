delete [SELECT Id FROM Account WHERE Name = 'AOBS-cpu_q'];
System.enqueueJob(new AsyncProbe__A__('cpu'));
obs = 'enqueued';
