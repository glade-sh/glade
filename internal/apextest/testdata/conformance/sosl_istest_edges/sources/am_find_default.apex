System.runAs(usr) { obs = '' + Search.find('FIND {amtest} RETURNING Account(Id, Name)').get('Account').size(); }
