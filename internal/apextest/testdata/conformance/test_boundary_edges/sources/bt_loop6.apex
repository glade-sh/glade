String r = '';
for (Integer i = 1; i <= 6; i++) {
    try { Database.executeBatch(new TbBatch__A__()); r += i + ':ok;'; } catch (Exception e) { r += i + ':' + e.getTypeName() + ':' + e.getMessage() + ';'; }
}
obs = r + Limits.getQueueableJobs();
