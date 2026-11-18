obs = SoslObs__A__.sizes(Search.query('FIND {capz*} RETURNING Account(Id ORDER BY Id LIMIT 2000), Contact(Id ORDER BY Id LIMIT 2000), Lead(Id ORDER BY Id LIMIT 2000)'));
