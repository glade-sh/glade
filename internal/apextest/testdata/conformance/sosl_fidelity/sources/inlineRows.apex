List<List<SObject>> buckets=[FIND 'Amberquartz' RETURNING Account(Id,AccountNumber ORDER BY Name DESC LIMIT 2),Contact(Id,Department ORDER BY LastName ASC LIMIT 1)];
obs=''+buckets[0].size()+'/'+buckets[1].size(); for(SObject rec:buckets[0]){obs+='|'+rec.get('AccountNumber');}for(SObject rec:buckets[1]){obs+='|'+rec.get('Department');}
