@IsTest
private class SyntheticTaxRoute {
    private class CalculationException extends Exception {}
    private class TaxProbe extends CartExtension.TaxCartCalculator {
        public override void calculate(CartExtension.CartCalculateCalculatorRequest request) {
            try {
                CartExtension.Cart cart = request.getCart();
                String state = cart.getCartDeliveryGroups().get(0).getDeliverToAddress().getState();
                CartExtension.CartItem item = cart.getCartItems().get(0);
                Decimal basis = item.getTotalPriceAfterAllAdjustments();
                if (basis == null) basis = item.getTotalListPrice();
                Decimal tax = basis * 0.1;
            } catch (Exception e) {
                throw new CalculationException(e.getTypeName());
            }
        }
    }
    private static String runCase(Boolean withAddress, Boolean total, Boolean net) {
        Account buyer = new Account(Name = 'Buyer'); insert buyer;
        WebStore store = new WebStore(Name = 'Store', OptionsCartCalculateEnabled = true); insert store;
        WebCart basket = new WebCart(Name = 'Basket', WebStoreId = store.Id, AccountId = buyer.Id, Status = 'Checkout'); insert basket;
        CartDeliveryGroup groupRecord = new CartDeliveryGroup(Name = 'Group', CartId = basket.Id);
        if (withAddress) { groupRecord.DeliverToCountry = 'US'; groupRecord.DeliverToState = 'CA'; }
        insert groupRecord;
        insert new CartItem(Name = 'Item', CartId = basket.Id, CartDeliveryGroupId = groupRecord.Id, Quantity = 3, Type = 'Product');
        CartExtension.Cart cart = CartExtension.CartTestUtil.getCart(basket.Id);
        CartExtension.CartItem item = cart.getCartItems().get(0);
        if (total) item.setTotalPrice(30);
        if (net) item.setNetUnitPrice(10);
        CartExtension.CartCalculateCalculatorRequest req = new CartExtension.CartCalculateCalculatorRequest(cart, CartExtension.OptionalBuyerActionDetails.empty());
        try { new TaxProbe().calculate(req); return 'success'; }
        catch (CalculationException e) { return 'wrapped=' + e.getMessage(); }
    }
    @IsTest static void unset() { System.assert(false, 'OBS|unset|' + runCase(true, false, false)); }
    @IsTest static void totalOnly() { System.assert(false, 'OBS|total|' + runCase(true, true, false)); }
    @IsTest static void netAndTotal() { System.assert(false, 'OBS|both|' + runCase(true, true, true)); }
    @IsTest static void noAddress() { System.assert(false, 'OBS|address|' + runCase(false, false, false)); }
}
