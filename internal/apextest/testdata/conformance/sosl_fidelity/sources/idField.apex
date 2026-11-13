SObject rec=Search.query('FIND {Amberquartz} RETURNING Account(Id)')[0][0]; obs=''+(rec.Id!=null)+'|'+rec.get('Phone');
