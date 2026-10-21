@IsTest private class CedarRenewalProbeTest {
    private static Map<String,Object> observe() {
        Account original = new Account(Name='Cedar original', Description='Processed');
        Account renewal = new Account(Name='Cedar renewal', Description='Processed');
        insert new List<Account>{original, renewal};
        Contact earlier = new Contact(LastName='Cedar shared product', AccountId=original.Id);
        insert earlier;
        Contact later = new Contact(LastName='Cedar shared product', AccountId=renewal.Id);
        insert later;
        insert new List<Task>{
            new Task(Subject='Cedar debit', WhatId=renewal.Id),
            new Task(Subject='Cedar credit', WhatId=renewal.Id)
        };
        List<Contact> natural = [SELECT Id, CreatedDate, AccountId FROM Contact WHERE LastName='Cedar shared product' ORDER BY CreatedDate DESC];
        Id naturalParent = natural[0].AccountId;
        Map<String,Object> rows = new Map<String,Object>{
            'natural_dates_equal'=>natural[0].CreatedDate==natural[1].CreatedDate,
            'natural_selected_original'=>natural[0].Id==earlier.Id,
            'natural_selected_count'=>[SELECT COUNT() FROM Task WHERE WhatId=:naturalParent]
        };
        Datetime tied = Datetime.newInstanceGmt(2003,4,5,6,7,8);
        Test.setCreatedDate(earlier.Id, tied);
        Test.setCreatedDate(later.Id, tied);
        List<Contact> lines = [SELECT Id, CreatedDate, AccountId, Account.Description FROM Contact WHERE LastName='Cedar shared product' ORDER BY CreatedDate DESC];
        Id selectedParent = lines[0].AccountId;
        rows.put('tied_dates_equal', lines[0].CreatedDate==lines[1].CreatedDate);
        rows.put('tied_selected_original', lines[0].Id==earlier.Id);
        rows.put('tied_selected_processed', lines[0].Account.Description=='Processed');
        rows.put('tied_selected_count', [SELECT COUNT() FROM Task WHERE WhatId=:selectedParent]);
        rows.put('explicit_renewal_count', [SELECT COUNT() FROM Task WHERE WhatId=:renewal.Id]);
        List<Contact> renewalOnly = [SELECT Id,AccountId FROM Contact WHERE LastName='Cedar shared product' AND AccountId=:renewal.Id ORDER BY CreatedDate DESC];
        rows.put('renewal_only_lines', renewalOnly.size());
        rows.put('renewal_only_count', [SELECT COUNT() FROM Task WHERE WhatId=:renewalOnly[0].AccountId]);
        return rows;
    }
    @IsTest static void observeRows() {
        System.assert(false, 'PROBE|'+JSON.serialize(observe()));
    }
    @IsTest static void assumesNewest() {
        Map<String,Object> rows = observe();
        System.assertEquals(2, rows.get('tied_selected_count'), 'Expected entries for renewal parent.');
    }
}
