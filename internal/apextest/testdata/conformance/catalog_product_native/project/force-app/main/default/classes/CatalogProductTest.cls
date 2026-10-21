@IsTest private class CatalogProductTest {
    @TestSetup static void setupObserver() {
        Profile profile = [SELECT Id FROM Profile WHERE Name='Standard User' AND UserLicense.Name='Salesforce' LIMIT 1];
        User observer = new User(
            Alias='catobs', Email='catalog@example.invalid', EmailEncodingKey='UTF-8',
            LastName='CatalogObserver', LanguageLocaleKey='en_US', LocaleSidKey='en_US',
            ProfileId=profile.Id, TimeZoneSidKey='America/Los_Angeles',
            Username='catalog.'+UserInfo.getOrganizationId()+'.'+DateTime.now().getTime()+'@example.invalid'
        );
        insert observer;
        PermissionSet access = [SELECT Id FROM PermissionSet WHERE Name='CatalogProductRead' LIMIT 1];
        insert new PermissionSetAssignment(AssigneeId=observer.Id, PermissionSetId=access.Id);
    }

    @IsTest static void productVisibility() {
        Product2 seed = new Product2(Name='Cedar Catalog', IsActive=true);
        insert seed;
        Id seedId = seed.Id;
        Integer systemCount = [SELECT count() FROM Product2 WHERE Id=:seedId];
        User observer = [SELECT Id FROM User WHERE LastName='CatalogObserver' LIMIT 1];
        Integer literalCount, dynamicCount, locatorCount = 0;
        System.runAs(observer) {
            literalCount = [SELECT count() FROM Product2 WHERE Id=:seedId WITH USER_MODE];
            String query = 'SELECT Id, Name FROM Product2 WHERE Id = :seedId WITH USER_MODE';
            dynamicCount = Database.query(query).size();
            Database.QueryLocatorIterator items = Database.getQueryLocator(query).iterator();
            while (items.hasNext()) { items.next(); locatorCount++; }
        }
        System.assert(false, 'OBSERVED|productVisibility|'+JSON.serialize(new Map<String,Object>{
            'systemCount'=>systemCount, 'literalCount'=>literalCount,
            'dynamicCount'=>dynamicCount, 'locatorCount'=>locatorCount
        }));
    }

    @IsTest static void linePriceDerivation() {
        Id bookId = Test.getStandardPricebookId();
        update new Pricebook2(Id=bookId, IsActive=true);
        Product2 seed = new Product2(Name='Birch Catalog', IsActive=true);
        insert seed;
        PricebookEntry entry = new PricebookEntry(Product2Id=seed.Id, Pricebook2Id=bookId, UnitPrice=30, IsActive=true);
        insert entry;
        Opportunity parent = new Opportunity(Name='Cedar Sale', StageName='Prospecting', CloseDate=Date.today(), Pricebook2Id=bookId);
        insert parent;
        OpportunityLineItem totalInput = new OpportunityLineItem(OpportunityId=parent.Id, PricebookEntryId=entry.Id, Quantity=2, TotalPrice=50);
        insert totalInput;
        OpportunityLineItem first = [SELECT Quantity, UnitPrice, TotalPrice FROM OpportunityLineItem WHERE Id=:totalInput.Id];
        Decimal firstAmount = [SELECT Amount FROM Opportunity WHERE Id=:parent.Id].Amount;
        update new OpportunityLineItem(Id=totalInput.Id, UnitPrice=20);
        OpportunityLineItem second = [SELECT Quantity, UnitPrice, TotalPrice FROM OpportunityLineItem WHERE Id=:totalInput.Id];
        Decimal secondAmount = [SELECT Amount FROM Opportunity WHERE Id=:parent.Id].Amount;
        OpportunityLineItem unitInput = new OpportunityLineItem(OpportunityId=parent.Id, PricebookEntryId=entry.Id, Quantity=3, UnitPrice=7);
        insert unitInput;
        OpportunityLineItem third = [SELECT Quantity, UnitPrice, TotalPrice FROM OpportunityLineItem WHERE Id=:unitInput.Id];
        Decimal finalAmount = [SELECT Amount FROM Opportunity WHERE Id=:parent.Id].Amount;
        System.assert(false, 'OBSERVED|linePriceDerivation|'+JSON.serialize(new Map<String,Object>{
            'inputUnitPrice'=>totalInput.UnitPrice,
            'firstUnitPrice'=>first.UnitPrice, 'firstTotalPrice'=>first.TotalPrice, 'firstAmount'=>firstAmount,
            'secondUnitPrice'=>second.UnitPrice, 'secondTotalPrice'=>second.TotalPrice, 'secondAmount'=>secondAmount,
            'thirdUnitPrice'=>third.UnitPrice, 'thirdTotalPrice'=>third.TotalPrice, 'finalAmount'=>finalAmount
        }));
    }

    @IsTest static void linePriceUpdates() {
        Id bookId = Test.getStandardPricebookId();
        update new Pricebook2(Id=bookId, IsActive=true);
        Product2 seed = new Product2(Name='Maple Catalog', IsActive=true);
        insert seed;
        PricebookEntry entry = new PricebookEntry(Product2Id=seed.Id, Pricebook2Id=bookId, UnitPrice=30, IsActive=true);
        insert entry;
        Opportunity parent = new Opportunity(Name='Maple Sale', StageName='Prospecting', CloseDate=Date.today(), Pricebook2Id=bookId);
        insert parent;
        OpportunityLineItem input = new OpportunityLineItem(OpportunityId=parent.Id, PricebookEntryId=entry.Id, Quantity=2, UnitPrice=10);
        insert input;
        update new OpportunityLineItem(Id=input.Id, Quantity=4);
        OpportunityLineItem quantityOnly = [SELECT Quantity, UnitPrice, TotalPrice FROM OpportunityLineItem WHERE Id=:input.Id];
        Decimal quantityAmount = [SELECT Amount FROM Opportunity WHERE Id=:parent.Id].Amount;
        update new OpportunityLineItem(Id=input.Id, TotalPrice=60);
        OpportunityLineItem totalOnly = [SELECT Quantity, UnitPrice, TotalPrice FROM OpportunityLineItem WHERE Id=:input.Id];
        Decimal totalAmount = [SELECT Amount FROM Opportunity WHERE Id=:parent.Id].Amount;
        OpportunityLineItem full = [SELECT Quantity, UnitPrice, TotalPrice FROM OpportunityLineItem WHERE Id=:input.Id];
        full.UnitPrice = 12;
        String fullResult = 'success';
        try { update full; }
        catch (DmlException ex) { fullResult = ex.getDmlStatusCode(0)+':'+ex.getDmlMessage(0); }
        OpportunityLineItem fullSaved = [SELECT Quantity, UnitPrice, TotalPrice FROM OpportunityLineItem WHERE Id=:input.Id];
        Decimal fullAmount = [SELECT Amount FROM Opportunity WHERE Id=:parent.Id].Amount;
        delete input;
        Decimal deletedAmount = [SELECT Amount FROM Opportunity WHERE Id=:parent.Id].Amount;
        List<OpportunityLineItem> pair = new List<OpportunityLineItem>{
            new OpportunityLineItem(OpportunityId=parent.Id, PricebookEntryId=entry.Id, Quantity=1, UnitPrice=3),
            new OpportunityLineItem(OpportunityId=parent.Id, PricebookEntryId=entry.Id, Quantity=2, UnitPrice=4)
        };
        insert pair;
        Decimal pairAmount = [SELECT Amount FROM Opportunity WHERE Id=:parent.Id].Amount;
        delete pair;
        Decimal finalAmount = [SELECT Amount FROM Opportunity WHERE Id=:parent.Id].Amount;
        System.assert(false, 'OBSERVED|linePriceUpdates|'+JSON.serialize(new Map<String,Object>{
            'quantityUnit'=>quantityOnly.UnitPrice, 'quantityTotal'=>quantityOnly.TotalPrice, 'quantityAmount'=>quantityAmount,
            'totalUnit'=>totalOnly.UnitPrice, 'totalTotal'=>totalOnly.TotalPrice, 'totalAmount'=>totalAmount,
            'fullResult'=>fullResult, 'fullUnit'=>fullSaved.UnitPrice, 'fullTotal'=>fullSaved.TotalPrice, 'fullAmount'=>fullAmount,
            'deletedAmount'=>deletedAmount, 'pairAmount'=>pairAmount, 'finalAmount'=>finalAmount
        }));
    }
}
