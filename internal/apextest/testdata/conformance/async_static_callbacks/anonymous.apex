System.enqueueJob(new StaticQueueProbe());
Database.executeBatch(new StaticBatchProbe());
Datetime due = Datetime.now().addSeconds(25);
System.schedule('Owned static schedule ' + Datetime.now().getTime(), due.format('s m H d M ? yyyy'), new StaticScheduleProbe());
