SObject rec=Search.query('FIND {Amberquartz} RETURNING Account(Name ORDER BY Name ASC LIMIT 1)')[0][0]; obs=''+(rec.Id!=null)+'|'+rec.get('Name');
