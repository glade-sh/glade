public class CedarAuditProbe {
    public static Map<String,Object> observe(Boolean testMode) {
        Account setup = [SELECT Id, CreatedDate FROM Account WHERE Name='Cedar setup' LIMIT 1];
        Account first = new Account(Name='Cedar first');
        insert first;
        Account second = new Account(Name='Cedar second');
        insert second;
        Set<Id> separateIds=new Set<Id>{first.Id,second.Id};
        Map<Id,Account> separate = new Map<Id,Account>([SELECT Id,CreatedDate,LastModifiedDate,SystemModstamp FROM Account WHERE Id IN :separateIds]);
        Datetime firstDate=separate.get(first.Id).CreatedDate;
        Datetime secondDate=separate.get(second.Id).CreatedDate;
        Account newest=[SELECT Id FROM Account WHERE Name LIKE 'Cedar %' ORDER BY CreatedDate DESC LIMIT 1];
        Map<String,Object> rows=new Map<String,Object>{
            'setup_before_first'=>setup.CreatedDate < firstDate,
            'separate_increasing'=>firstDate < secondDate,
            'separate_delta_ms'=>secondDate.getTime()-firstDate.getTime(),
            'newest_is_second'=>newest.Id==second.Id,
            'first_fraction_ms'=>Math.mod(firstDate.getTime(),1000),
            'second_fraction_ms'=>Math.mod(secondDate.getTime(),1000)
        };
        List<Account> batch=new List<Account>{new Account(Name='Cedar batch first'),new Account(Name='Cedar batch second')};
        insert batch;
        Set<Id> batchIds=new Set<Id>{batch[0].Id,batch[1].Id};
        List<Account> tied=[SELECT Id,CreatedDate FROM Account WHERE Id IN :batchIds ORDER BY CreatedDate DESC];
        List<Account> tiedId=[SELECT Id FROM Account WHERE Id IN :batchIds ORDER BY CreatedDate DESC,Id DESC];
        rows.put('list_dates_equal',tied[0].CreatedDate==tied[1].CreatedDate);
        rows.put('list_tie_first_inserted',tied[0].Id==batch[0].Id);
        rows.put('list_id_desc_second',tiedId[0].Id==batch[1].Id);
        Account parent=new Account(Name='Cedar trigger parent');
        insert parent;
        parent=[SELECT Id,CreatedDate FROM Account WHERE Id=:parent.Id];
        Contact child=[SELECT Id,CreatedDate FROM Contact WHERE AccountId=:parent.Id LIMIT 1];
        rows.put('trigger_child_before_parent',child.CreatedDate<parent.CreatedDate);
        rows.put('trigger_dates_equal',child.CreatedDate==parent.CreatedDate);
        second.Description='Updated';
        update second;
        Account updated=[SELECT CreatedDate,LastModifiedDate,SystemModstamp FROM Account WHERE Id=:second.Id];
        rows.put('update_created_preserved',updated.CreatedDate==secondDate);
        rows.put('update_modified_increasing',updated.LastModifiedDate>separate.get(second.Id).LastModifiedDate);
        rows.put('update_modstamp_increasing',updated.SystemModstamp>separate.get(second.Id).SystemModstamp);
        rows.put('update_modified_equals_modstamp',updated.LastModifiedDate==updated.SystemModstamp);
        rows.put('update_fraction_ms',Math.mod(updated.LastModifiedDate.getTime(),1000));
        if(testMode) {
            Datetime explicitDate=Datetime.newInstanceGmt(2003,4,5,6,7,8);
            Test.setCreatedDate(first.Id,explicitDate);
            first.Description='Explicit date retained';
            update first;
            first=[SELECT CreatedDate FROM Account WHERE Id=:first.Id];
            rows.put('explicit_date_preserved',first.CreatedDate==explicitDate);
        }
        Datetime cutoff=Datetime.now();
        rows.put('audit_not_future',[SELECT COUNT() FROM Account WHERE Name LIKE 'Cedar %' AND LastModifiedDate>:cutoff]==0);
        return rows;
    }
}
