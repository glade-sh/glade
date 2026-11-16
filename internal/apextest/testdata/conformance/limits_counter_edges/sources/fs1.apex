String b = Limits.getSoslQueries() + '/' + Limits.getLimitSoslQueries() + '/' + Limits.getFindSimilarCalls() + '/' + Limits.getLimitFindSimilarCalls();
Search.query('FIND {copper} RETURNING Account(Id)');
String a1 = Limits.getSoslQueries() + '/' + Limits.getLimitSoslQueries() + '/' + Limits.getFindSimilarCalls() + '/' + Limits.getLimitFindSimilarCalls();
Search.find('FIND {copper} RETURNING Account(Id)');
String a2 = Limits.getSoslQueries() + '/' + Limits.getLimitSoslQueries() + '/' + Limits.getFindSimilarCalls() + '/' + Limits.getLimitFindSimilarCalls();
List<List<SObject>> r = [FIND 'copper' RETURNING Account(Id)];
String a3 = Limits.getSoslQueries() + '/' + Limits.getLimitSoslQueries() + '/' + Limits.getFindSimilarCalls() + '/' + Limits.getLimitFindSimilarCalls();
obs = b + '>' + a1 + '>' + a2 + '>' + a3;
