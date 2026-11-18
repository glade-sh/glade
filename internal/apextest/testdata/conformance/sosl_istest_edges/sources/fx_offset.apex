Test.setFixedSearchResults(new List<Id>{ a3.Id, a1.Id, a2.Id });
List<List<SObject>> r = Search.query('FIND {nomatch} RETURNING Account(Name ORDER BY Name LIMIT 1 OFFSET 1)');
obs = SoslObs__A__.fmt(r);
