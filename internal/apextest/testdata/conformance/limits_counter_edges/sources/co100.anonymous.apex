Http h = new Http();
HttpRequest r = new HttpRequest();
r.setEndpoint('https://example.com/');
r.setMethod('GET');
for (Integer i = 0; i < 100; i++) { h.send(r); }
obs = Limits.getCallouts() + '|' + Limits.getLimitCallouts();
