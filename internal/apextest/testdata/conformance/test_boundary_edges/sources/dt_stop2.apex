Test.startTest();
Test.stopTest();
String r = 'first-ok';
try { Test.stopTest(); r += '|second-ok'; } catch (Exception e) { r += '|caught:' + e.getTypeName() + ':' + e.getMessage(); }
obs = r;
