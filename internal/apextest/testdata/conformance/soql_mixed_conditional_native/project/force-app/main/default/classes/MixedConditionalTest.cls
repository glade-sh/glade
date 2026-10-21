@IsTest private class MixedConditionalTest {
    @IsTest static void mixedArms() {
        Account seed = new Account(Name='Cedar');
        insert seed;
        List<Account> records = new List<Account>{seed};
        Boolean chooseQuery = true;
        Account first = chooseQuery ? [SELECT Id FROM Account WHERE Id=:seed.Id LIMIT 1] : records[0];
        chooseQuery = false;
        Account second = chooseQuery ? [SELECT Id FROM Account WHERE Id=:seed.Id LIMIT 1] : records[0];
        Boolean chooseRecord = true;
        Account third = chooseRecord ? records[0] : [SELECT Id FROM Account WHERE Id=:seed.Id LIMIT 1];
        chooseRecord = false;
        Account fourth = chooseRecord ? records[0] : [SELECT Id FROM Account WHERE Id=:seed.Id LIMIT 1];
        System.assert(false, 'OBSERVED|mixedArms|'+JSON.serialize(new Map<String,Object>{
            'first'=>first.Id==seed.Id, 'second'=>second.Id==seed.Id,
            'third'=>third.Id==seed.Id, 'fourth'=>fourth.Id==seed.Id
        }));
    }

    @IsTest static void emptyAndNullControls() {
        Account seed = new Account(Name='Birch');
        insert seed;
        Boolean chooseRecord = true;
        Account first = chooseRecord ? seed : [SELECT Id FROM Account WHERE Name='NoSuchConditionalRecord' LIMIT 1];
        chooseRecord = false;
        Account second = chooseRecord ? [SELECT Id FROM Account WHERE Name='NoSuchConditionalRecord' LIMIT 1] : seed;
        String emptyResult = 'no exception';
        try {
            Account third = chooseRecord ? seed : [SELECT Id FROM Account WHERE Name='NoSuchConditionalRecord' LIMIT 1];
        } catch (Exception ex) {
            emptyResult = ex.getTypeName()+':'+ex.getMessage();
        }
        Boolean chooseQuery = true;
        Account fourth = chooseQuery ? [SELECT Id FROM Account WHERE Id=:seed.Id LIMIT 1] : null;
        chooseQuery = false;
        Account fifth = chooseQuery ? [SELECT Id FROM Account WHERE Id=:seed.Id LIMIT 1] : null;
        System.assert(false, 'OBSERVED|emptyAndNullControls|'+JSON.serialize(new Map<String,Object>{
            'first'=>first.Id==seed.Id, 'second'=>second.Id==seed.Id,
            'emptyResult'=>emptyResult, 'fourth'=>fourth.Id==seed.Id, 'fifth'=>fifth==null
        }));
    }
}
