SObject rec=Search.query('FIND {Amberquartz} RETURNING Account')[0][0]; obs=''+(rec.Id!=null)+'|'+rec.get('Phone');
