List<String> markers = new List<String>();
for (Account row : [SELECT Name FROM Account ORDER BY Name]) markers.add(row.Name);
System.debug('OBS|' + JSON.serialize(markers));
