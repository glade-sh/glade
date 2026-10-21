@IsTest private class DeliveryCurrencyTest {
    @IsTest static void inherited_insert() {
        Account buyer = new Account(Name='Synthetic Buyer');
        insert as system buyer;
        Order purchase = new Order(AccountId=buyer.Id, EffectiveDate=Date.today(), Status='Draft');
        insert as system purchase;
        OrderDeliveryGroup delivery = new OrderDeliveryGroup(OrderId=purchase.Id, DeliverToName='Synthetic Recipient');
        Database.SaveResult result = Database.insert(delivery, false, AccessLevel.SYSTEM_MODE);
        Map<String,Object> observed = new Map<String,Object>{'success'=>result.isSuccess()};
        if (result.isSuccess()) {
            delivery = [SELECT CurrencyIsoCode, DeliverToName FROM OrderDeliveryGroup WHERE Id=:delivery.Id WITH SYSTEM_MODE];
            purchase = [SELECT CurrencyIsoCode FROM Order WHERE Id=:purchase.Id WITH SYSTEM_MODE];
            observed.put('currencyInherited', delivery.CurrencyIsoCode == purchase.CurrencyIsoCode);
            delivery.DeliverToName='Updated Recipient';
            update as system delivery;
            observed.put('updated', [SELECT DeliverToName FROM OrderDeliveryGroup WHERE Id=:delivery.Id WITH SYSTEM_MODE].DeliverToName == 'Updated Recipient');
        } else {
            observed.put('error', String.valueOf(result.getErrors()[0].getStatusCode()));
        }
        System.assert(false, 'OBSERVED|inherited_insert|'+JSON.serialize(observed));
    }
    @IsTest static void field_flags() {
        Schema.DescribeFieldResult field=OrderDeliveryGroup.CurrencyIsoCode.getDescribe();
        System.assert(false, 'OBSERVED|field_flags|'+JSON.serialize(new Map<String,Object>{'createable'=>field.isCreateable(),'updateable'=>field.isUpdateable(),'calculated'=>field.isCalculated(),'defaulted'=>field.isDefaultedOnCreate(),'hasDefault'=>field.getDefaultValue()!=null}));
    }
}
