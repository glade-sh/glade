List<List<SObject>> rows=[FIND 'Amberquartz' RETURNING Account(Id WHERE Id=null) WITH SYSTEM_MODE];obs='ok';
