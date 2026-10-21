public class OrderProductProbe {
    public static String sample() {
        Account owner = new Account(Name = 'Synthetic account');
        insert as system owner;
        Product2 product = new Product2(Name = 'Synthetic tool', Family = 'Synthetic Tools', IsActive = true);
        insert as system product;
        Id book = Test.isRunningTest() ? Test.getStandardPricebookId() : [SELECT Id FROM Pricebook2 WHERE IsStandard = true LIMIT 1].Id;
        PricebookEntry entry = new PricebookEntry(Pricebook2Id = book, Product2Id = product.Id, UnitPrice = 24, IsActive = true);
        insert as system entry;
        Order purchase = new Order(AccountId = owner.Id, Pricebook2Id = book, EffectiveDate = Date.today(), Status = 'Draft');
        insert as system purchase;
        OrderItem line = new OrderItem(OrderId = purchase.Id, PricebookEntryId = entry.Id, Quantity = 2, UnitPrice = 24);
        insert as system line;
        OrderItem stored = [SELECT Product2Id, Product2.Family, PricebookEntry.Product2Id, TotalPrice FROM OrderItem WHERE Id = :line.Id];
        Map<String,Object> observed = new Map<String,Object>{'productId' => stored.Product2Id == product.Id, 'family' => stored.Product2.Family, 'entryProductId' => stored.PricebookEntry.Product2Id == product.Id, 'total' => stored.TotalPrice};
        observed.put('orderTotal', [SELECT TotalAmount FROM Order WHERE Id = :purchase.Id].TotalAmount);
        line.Quantity = 3;
        line.UnitPrice = 25;
        update as system line;
        OrderItem updated = [SELECT TotalPrice FROM OrderItem WHERE Id = :line.Id];
        observed.put('updatedTotal', updated.TotalPrice);
        observed.put('updatedOrderTotal', [SELECT TotalAmount FROM Order WHERE Id = :purchase.Id].TotalAmount);
        delete as system line;
        observed.put('deletedOrderTotal', [SELECT TotalAmount FROM Order WHERE Id = :purchase.Id].TotalAmount);
        delete as system purchase;
        delete as system entry;
        delete as system product;
        delete as system owner;
        return JSON.serialize(observed);
    }
}
