obs = SoslObs__A__.fmt(Search.query('FIND {copper} IN NAME FIELDS RETURNING Contact(Name), Lead(Name), Case(Subject), Task(Subject)'));
