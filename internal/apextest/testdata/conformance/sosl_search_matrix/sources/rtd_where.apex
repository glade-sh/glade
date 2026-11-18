List<List<SObject>> r = Search.query('FIND {copper OR zork} RETURNING Account(Name WHERE Name LIKE \'copper%\'), Account(Name WHERE Name LIKE \'zork%\')'); obs = r.size() + '|' + SoslObs__A__.fmt(r);
