@IsTest
private class SyntheticTaxProbe {
    @IsTest static void observe() {
        Account buyer = new Account(Name = 'Buyer');
        insert buyer;
        WebStore store = new WebStore(Name = 'Store', OptionsCartCalculateEnabled = true);
        insert store;
        WebCart basket = new WebCart(Name = 'Basket', WebStoreId = store.Id, AccountId = buyer.Id, Status = 'Checkout');
        insert basket;
        CartDeliveryGroup groupRecord = new CartDeliveryGroup(Name = 'Group', CartId = basket.Id, DeliverToCountry = 'US', DeliverToState = 'CA');
        insert groupRecord;
        insert new CartItem(Name = 'Item', CartId = basket.Id, CartDeliveryGroupId = groupRecord.Id, Quantity = 3, Type = 'Product');
        CartExtension.Cart loaded = CartExtension.CartTestUtil.getCart(basket.Id);
        CartExtension.CartItem item = loaded.getCartItems().get(0);
        Decimal basis = item.getTotalPriceAfterAllAdjustments();
        if (basis == null) basis = item.getTotalListPrice();
        String out = 'adjustedNull=' + (item.getTotalPriceAfterAllAdjustments() == null)
            + ',listNull=' + (item.getTotalListPrice() == null)
            + ',quantity=' + item.getQuantity()
            + ',addressNull=' + (loaded.getCartDeliveryGroups().get(0).getDeliverToAddress() == null);
        try { Decimal tax = basis * 0.1; out += ',tax=' + tax; }
        catch (Exception e) { out += ',exception=' + e.getTypeName(); }
        System.assert(false, 'OBS|' + out);
    }
}
