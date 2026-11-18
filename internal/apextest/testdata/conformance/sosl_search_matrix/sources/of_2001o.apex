obs = SoslObs__A__.fmt(Search.query('FIND {zork*} RETURNING Account(Name ORDER BY Name LIMIT 1 OFFSET 2001)'));
