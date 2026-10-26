Set<String> names = new Set<String>{'cedar first transaction', 'cedar second transaction'};
for (CronTrigger job : [SELECT Id FROM CronTrigger WHERE CronJobDetail.Name IN :names]) System.abortJob(job.Id);
