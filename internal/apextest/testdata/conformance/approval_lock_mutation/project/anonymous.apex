
  String out='';
  Account a=new Account(Name='Update probe'); insert a; Approval.lock(a); a.Name='Changed'; Database.SaveResult u=Database.update(a,false); out += 'update='+String.valueOf(u.isSuccess())+':'+(u.isSuccess()?'NONE':String.valueOf(u.getErrors()[0].getStatusCode()))+';';
  Account b=new Account(Name='Delete probe'); insert b; Approval.lock(b); Database.DeleteResult d=Database.delete(b,false); out += 'delete='+String.valueOf(d.isSuccess())+':'+(d.isSuccess()?'NONE':String.valueOf(d.getErrors()[0].getStatusCode()))+';';
  Account c=new Account(Name='Upsert probe'); insert c; Approval.lock(c); c.Name='Changed'; Database.UpsertResult v=Database.upsert(c,false); out += 'upsert='+String.valueOf(v.isSuccess())+':'+(v.isSuccess()?'NONE':String.valueOf(v.getErrors()[0].getStatusCode()))+';';
  System.debug('MUTATE_ROWS|'+out);
