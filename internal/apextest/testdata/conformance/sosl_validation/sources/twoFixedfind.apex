obs=String.valueOf(Search.find('FIND {xy} RETURNING Account(Id)').get('Account').size());
