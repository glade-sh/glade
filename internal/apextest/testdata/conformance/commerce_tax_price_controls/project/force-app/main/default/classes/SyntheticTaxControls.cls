@IsTest
private class SyntheticTaxControls {
    private static CartExtension.Cart setupCart(Boolean withAddress) {
        Account buyer = new Account(Name = 'Buyer'); insert buyer;
        WebStore store = new WebStore(Name = 'Store', OptionsCartCalculateEnabled = true); insert store;
        WebCart basket = new WebCart(Name = 'Basket', WebStoreId = store.Id, AccountId = buyer.Id, Status = 'Checkout'); insert basket;
        CartDeliveryGroup groupRecord = new CartDeliveryGroup(Name = 'Group', CartId = basket.Id);
        if (withAddress) { groupRecord.DeliverToCountry = 'US'; groupRecord.DeliverToState = 'CA'; }
        insert groupRecord;
        insert new CartItem(Name = 'Item', CartId = basket.Id, CartDeliveryGroupId = groupRecord.Id, Quantity = 3, Type = 'Product');
        return CartExtension.CartTestUtil.getCart(basket.Id);
    }
    private static String result(CartExtension.CartItem item) {
        Decimal basis = item.getTotalPriceAfterAllAdjustments();
        if (basis == null) basis = item.getTotalListPrice();
        String result = 'adjustedNull=' + (item.getTotalPriceAfterAllAdjustments() == null)
            + ',listNull=' + (item.getTotalListPrice() == null);
        try { Decimal tax = basis * 0.1; result += ',tax=' + tax; }
        catch (Exception e) { result += ',exception=' + e.getTypeName(); }
        return result;
    }
    @IsTest static void totalPrice() {
        CartExtension.CartItem item = setupCart(true).getCartItems().get(0);
        item.setTotalPrice(30);
        System.assert(false, 'OBS|total|' + result(item));
    }
    @IsTest static void netAndTotal() {
        CartExtension.CartItem item = setupCart(true).getCartItems().get(0);
        item.setTotalPrice(30);
        item.setNetUnitPrice(10);
        System.assert(false, 'OBS|both|' + result(item));
    }
    @IsTest static void absentAddress() {
        CartExtension.Cart cart = setupCart(false);
        String result;
        try { result = 'state=' + cart.getCartDeliveryGroups().get(0).getDeliverToAddress().getState(); }
        catch (Exception e) { result = 'exception=' + e.getTypeName(); }
        System.assert(false, 'OBS|address|' + result);
    }
}
