String r = 'none';
try { Test.stopTest(); r = 'ok'; } catch (Exception e) { r = 'caught:' + e.getTypeName() + ':' + e.getMessage(); }
obs = r;
