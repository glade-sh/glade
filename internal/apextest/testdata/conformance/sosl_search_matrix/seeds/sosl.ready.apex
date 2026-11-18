List<List<SObject>> a = Search.query('FIND {zork OR quux OR copper OR Coppers} RETURNING Account(Name)');
List<List<SObject>> b = Search.query('FIND {copper} RETURNING Contact(Name), Lead(Name), Case(Subject), Task(Subject)');
List<List<SObject>> c = Search.query('FIND {zork} RETURNING Document(Name)');
List<List<SObject>> d = Search.query('FIND {copper} IN ALL FIELDS RETURNING Account(Name), Contact(Name), Lead(Name)');
obs = SoslObs67.fmt(a) + '#' + SoslObs67.fmt(b) + '#' + SoslObs67.fmt(c) + '#' + SoslObs67.fmt(d);
