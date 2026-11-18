Test.setFixedSearchResults(new List<Id>{ a1.Id, a1.Id });
obs = SoslObs__A__.fmt(Search.query('FIND {nomatch} RETURNING Account(Name)'));
