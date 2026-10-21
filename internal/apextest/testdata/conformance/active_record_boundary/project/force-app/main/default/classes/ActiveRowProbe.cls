public class ActiveRowProbe {
    public static String innerStatus;
    public static Boolean busy = false;
    public static void handle(Account current) {
        if (busy || current.Description == null || !current.Description.startsWith('probe_')) return;
        busy = true;
        try {
            Account target = new Account(Id=current.Id,Name='Owned nested update');
            if (current.Description == 'probe_delete') {
                Database.DeleteResult result = Database.delete(target,false);
                innerStatus = result.isSuccess() ? 'success' : String.valueOf(result.getErrors()[0].getStatusCode());
            } else if (current.Description == 'probe_statement') {
                delete target;
                innerStatus = 'success';
            } else {
                Database.SaveResult result = Database.update(target,false);
                innerStatus = result.isSuccess() ? 'success' : String.valueOf(result.getErrors()[0].getStatusCode());
            }
        } catch (DmlException error) { innerStatus = String.valueOf(error.getDmlStatusCode(0)); }
        busy = false;
    }
    public static Map<String,Object> observe() {
        Map<String,Object> observed = new Map<String,Object>();
        for (String action : new List<String>{'probe_delete','probe_statement','probe_update'}) {
            Account row = new Account(Name='Owned active row');
            insert row;
            innerStatus = null;
            row.Description = action;
            String outerStatus = 'success';
            try { update row; }
            catch (DmlException error) { outerStatus = String.valueOf(error.getDmlStatusCode(0)); }
            observed.put(action,new Map<String,Object>{'inner'=>innerStatus,'outer'=>outerStatus,'remaining'=>[SELECT COUNT() FROM Account WHERE Id=:row.Id]});
        }
        return observed;
    }
}
