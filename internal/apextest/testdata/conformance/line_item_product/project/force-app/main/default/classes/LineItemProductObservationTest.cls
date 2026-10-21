@IsTest
private class LineItemProductObservationTest {
    @IsTest static void productLink() {
        Product2 product = new Product2(Name='Ownedcomponent',IsActive=true);
        insert product;
        Id pricebookId = Test.getStandardPricebookId();
        PricebookEntry price = new PricebookEntry(Pricebook2Id=pricebookId,Product2Id=product.Id,UnitPrice=20,IsActive=true);
        insert price;
        Opportunity deal = new Opportunity(Name='Owned deal',StageName='Prospecting',CloseDate=Date.today().addDays(5),Pricebook2Id=pricebookId);
        insert deal;
        OpportunityLineItem item = new OpportunityLineItem(OpportunityId=deal.Id,PricebookEntryId=price.Id,Quantity=1,UnitPrice=20);
        insert item;
        OpportunityLineItem saved = [SELECT Product2Id,Product2.Name FROM OpportunityLineItem WHERE Id=:item.Id];
        Map<String,Object> populated = saved.getPopulatedFieldsAsMap();
        SObject related = (SObject)populated.get('Product2');
        Map<String,Object> observation = new Map<String,Object>{
            'productIDequalsInserted'=>saved.Product2Id==product.Id,
            'nestedName'=>saved.Product2.Name,
            'relationshipInMap'=>populated.containsKey('Product2'),
            'mapNestedName'=>related==null ? null : related.get('Name')
        };
        System.assert(false,'OBSERVED|productLink|'+JSON.serialize(observation));
    }
}
