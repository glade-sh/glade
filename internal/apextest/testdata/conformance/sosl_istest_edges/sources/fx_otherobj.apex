Test.setFixedSearchResults(new List<Id>{ c1.Id, a1.Id });
obs = SoslObs__A__.fmt(Search.query('FIND {nomatch} RETURNING Account(Name)')) + '|' + SoslObs__A__.fmt(Search.query('FIND {nomatch} RETURNING Contact(Name)'));
