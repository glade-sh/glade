Map<String,Object> markers=new Map<String,Object>(); for(Account row:[SELECT Name,Description FROM Account WHERE Name LIKE 'batch%' ORDER BY Name]) markers.put(row.Name, JSON.deserializeUntyped(row.Description));
System.debug('OBS|' + JSON.serialize(markers));
