String queueId = System.enqueueJob(new IdentityQueueProbe());
String batchId = Database.executeBatch(new IdentityBatchProbe());
Datetime due = Datetime.now().addSeconds(25);
String cron = due.format('s m H d M ? yyyy');
String scheduleId = System.schedule('Owned identity schedule ' + Datetime.now().getTime(), cron, new IdentityScheduleProbe());
insert new Account(Name = 'returnIds', Description = JSON.serialize(new List<String>{queueId, batchId, scheduleId}));
