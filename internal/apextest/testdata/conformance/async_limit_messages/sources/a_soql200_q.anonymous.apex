delete [SELECT Id FROM Account WHERE Name = 'AOBS-soql200_q'];
System.enqueueJob(new AsyncProbe__A__('soql200'));
obs = 'enqueued';
