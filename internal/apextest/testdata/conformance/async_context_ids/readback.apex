List<String> ids = (List<String>) JSON.deserialize([SELECT Description FROM Account WHERE Name = 'returnIds'].Description, List<String>.class);
System.debug('OBS|' + JSON.serialize(AsyncIdentityProbe.observe(ids[0], ids[1], ids[2])));
