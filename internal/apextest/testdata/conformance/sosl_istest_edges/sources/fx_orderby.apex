Test.setFixedSearchResults(new List<Id>{ a3.Id, a1.Id, a2.Id });
List<List<SObject>> r = Search.query('FIND {nomatch} RETURNING Account(Name ORDER BY Name)');
List<String> n = new List<String>(); for (SObject o : r[0]) { n.add((String) o.get('Name')); }
obs = String.join(n, ',');
