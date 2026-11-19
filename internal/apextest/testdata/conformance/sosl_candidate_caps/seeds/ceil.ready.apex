List<List<SObject>> r = Search.query('FIND {ceilz*} RETURNING Account(Id WHERE Id != null LIMIT 2000)');
obs = SoslObs67.sizes(r);
