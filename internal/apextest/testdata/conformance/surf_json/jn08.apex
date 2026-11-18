JSONParser p = JSON.createParser('{"a":1}');
p.nextToken();
System.debug('OBS|pos=' + p.getPosition());
