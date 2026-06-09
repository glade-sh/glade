public class PerfRisk implements Database.Batchable<SObject>, Queueable {
    @AuraEnabled
    public static List<Account> uncachedAccounts(List<Id> ids) {
        List<Account> out = new List<Account>();
        for (Id accountId : ids) {
            out.add([SELECT Id, Name, Description FROM Account WHERE Id = :accountId]);
        }
        Schema.getGlobalDescribe();
        Schema.getGlobalDescribe();
        return out;
    }

    @AuraEnabled
    public static void voidAuraMethod() {
        // Void return methods cannot be cached.
    }

    @AuraEnabled
    public static void createProvider(String name) {
        ProviderFacade.createProvider(name);
    }

    @AuraEnabled
    public static void toggleLogMonitoring(Boolean enabled) {
        ProviderFacade.toggleLogMonitoring(enabled);
    }

    @AuraEnabled
    public static void activateSubscription(Id providerId) {
        ProviderFacade.activateSubscription(providerId);
    }

    @InvocableMethod
    public static void invocableWork(List<Id> ids) {
        for (Id accountId : ids) {
            update new Account(Id = accountId, Description = 'changed');
        }
    }

    public static void falsePositiveTests(List<Id> ids) {
        String sqlExample = 'for (SObject row : rows) { update row; }';
        String commented = '// for (Id id : ids) { [SELECT Id FROM Account]; }';
        if (ids == null || ids.isEmpty()) {
            return;
        }
        if (Test.isRunningTest()) {
            for (Id accountId : ids) {
                List<Account> ignored = [SELECT Id, Name FROM Account WHERE Id = :accountId];
                ignored.size();
            }
        }
    }

    public static void skipCustomShadows(List<Id> ids) {
        FakeDatabase db = new FakeDatabase();
        FakeQueueable queueable = new FakeQueueable();
        for (Id id : ids) {
            db.insert(new Account(Id = id));
            queueable.enqueueJob(new Account());
            db.update(new Account(Id = id));
        }
    }

    public Database.QueryLocator start(Database.BatchableContext context) {
        return Database.getQueryLocator('SELECT Id, Name FROM Account');
    }

    public void execute(Database.BatchableContext context, List<SObject> scope) {
        for (SObject row : scope) {
            System.enqueueJob(new PerfRisk());
        }
    }

    public void finish(Database.BatchableContext context) {
    }

    public void execute(QueueableContext context) {
    }

    public PageReference save() {
        List<Account> accounts = [SELECT Id, Name FROM Account];
        update accounts;
        return null;
    }
}

class FakeDatabase {
    public static void insert(Account a) {}
    public static void update(Account a) {}
}

class ProviderFacade {
    public static void createProvider(String name) {}
    public static void toggleLogMonitoring(Boolean enabled) {}
    public static void activateSubscription(Id providerId) {}
}

class FakeQueueable {
    public void enqueueJob(Object o) {}
}

class PerfRiskSelectivityTests {
    public static void selectivityAndAggregationScans() {
        List<Account> accounts = [
            SELECT Id, Name
            FROM Account
            WHERE Name LIKE '%Acme'
        ];

        List<Account> fields = [SELECT FIELDS(ALL) FROM Account WHERE Id != null];

        List<Account> manyFields = [SELECT
            Id, Name, Description, Phone, BillingStreet, BillingCity, BillingState, BillingPostalCode, BillingCountry,
            ShippingStreet, ShippingCity, ShippingState, ShippingPostalCode, ShippingCountry,
            CreatedDate, SystemModstamp, OwnerId, Type, Industry, AnnualRevenue, Website, LastName, FirstName,
            Name, ExternalId__c
            FROM Account WHERE Id != null];

        List<Account> parentChild = [SELECT Id, Name, (SELECT Id FROM Contacts) FROM Account];

        List<Account> formulaWhere = [SELECT Id FROM Account WHERE CALENDAR_MONTH(CreatedDate) = 12];

        List<Account> orderBy = [SELECT Id FROM Account WHERE OwnerId != null ORDER BY UnindexedText__c];

        Integer countAll = [SELECT COUNT() FROM Account];
        Integer countById = [SELECT COUNT(Id) FROM Account];
    }
}

public class QueueableChainStart implements Queueable {
    public void execute(QueueableContext context) {
        System.enqueueJob(new QueueableChainStep2());
    }
}

public class QueueableChainStep2 implements Queueable {
    public void execute(QueueableContext context) {
        System.enqueueJob(new QueueableChainStep3());
    }
}

public class QueueableChainStep3 implements Queueable {
    public void execute(QueueableContext context) {
        System.enqueueJob(new QueueableChainStep4());
    }
}

public class QueueableChainStep4 implements Queueable {
    public void execute(QueueableContext context) {
        System.enqueueJob(new QueueableChainStep5());
    }
}

public class QueueableChainStep5 implements Queueable {
    public void execute(QueueableContext context) {
        System.enqueueJob(new QueueableChainFinal());
    }
}

public class QueueableChainFinal implements Queueable {
    public void execute(QueueableContext context) {}
}

public class QueueableCycleA implements Queueable {
    public void execute(QueueableContext context) {
        System.enqueueJob(new QueueableCycleB());
    }
}

public class QueueableCycleB implements Queueable {
    public void execute(QueueableContext context) {
        System.enqueueJob(new QueueableCycleC());
    }
}

public class QueueableCycleC implements Queueable {
    public void execute(QueueableContext context) {
        System.enqueueJob(new QueueableCycleA());
    }
}

public class BatchChainExecute implements Database.Batchable<SObject> {
    public Database.QueryLocator start(Database.BatchableContext context) {
        return Database.getQueryLocator('SELECT Id FROM Account WHERE Id != null');
    }

    public void execute(Database.BatchableContext context, List<SObject> scope) {
        Database.executeBatch(new BatchChainFinish());
    }

    public void finish(Database.BatchableContext context) {
    }
}

public class BatchChainFinish implements Database.Batchable<SObject> {
    public Database.QueryLocator start(Database.BatchableContext context) {
        return Database.getQueryLocator('SELECT Id FROM Account WHERE Id != null');
    }

    public void execute(Database.BatchableContext context, List<SObject> scope) {
    }

    public void finish(Database.BatchableContext context) {
        Database.executeBatch(new BatchChainFinish());
    }
}

public class BatchExecuteQueueableBridge implements Database.Batchable<SObject> {
    public Database.QueryLocator start(Database.BatchableContext context) {
        return Database.getQueryLocator('SELECT Id FROM Account WHERE Id != null');
    }

    public void execute(Database.BatchableContext context, List<SObject> scope) {
        System.enqueueJob(new FutureChainJob());
    }

    public void finish(Database.BatchableContext context) {
    }
}

public class FutureChainJob implements Queueable {
    public void execute(QueueableContext context) {
    }
}

public class RecursiveScheduler implements Schedulable {
    public void execute(SchedulableContext context) {
        System.schedule('recursive-scan', '0 0 0 1 1 ? 2036', new RecursiveScheduler());
    }
}

public class FutureRisk {
    @future
    public static void futureOnly() {
        Account account = new Account(Name = 'AsyncOnly');
        insert account;
    }

    @future(callout=true)
    public static void futureCalloutWithDml() {
        List<String> names = new List<String>{'alpha'};
        insert new Account(Name = names[0]);
    }
}
