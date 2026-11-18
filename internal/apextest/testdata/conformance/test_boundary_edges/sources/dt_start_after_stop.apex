Test.startTest();
Test.stopTest();
String r = 'ok';
try { Test.startTest(); Test.stopTest(); r += '|again-ok'; } catch (Exception e) { r += '|caught:' + e.getTypeName() + ':' + e.getMessage(); }
obs = r;
