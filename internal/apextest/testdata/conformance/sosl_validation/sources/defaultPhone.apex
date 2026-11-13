SObject rec=Search.query('FIND {amberquartz} RETURNING Account')[0][0];obs=String.valueOf(rec.get('Phone'));
