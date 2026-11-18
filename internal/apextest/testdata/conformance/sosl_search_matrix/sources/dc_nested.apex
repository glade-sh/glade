obs = SoslObs__A__.fmt(Search.query('FIND {copper} RETURNING Account(Name WHERE Id IN (SELECT AccountId FROM Contact))'));
