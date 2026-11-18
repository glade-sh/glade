String r = '';
try { Test.enqueueBatchJobs(5); r = 'ok'; } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); }
obs = r;
