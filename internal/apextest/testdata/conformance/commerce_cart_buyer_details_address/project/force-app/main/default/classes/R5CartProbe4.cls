@IsTest
private class R5CartProbe4 {
  private static CartExtension.Cart mk() {
    Account a = new Account(Name = 'acc'); insert a;
    WebStore ws = new WebStore(Name = 'ws', OptionsCartCalculateEnabled = true); insert ws;
    WebCart wc = new WebCart(Name = 'cart', WebStoreId = ws.Id, AccountId = a.Id, Status = 'Checkout'); insert wc;
    CartDeliveryGroup g = new CartDeliveryGroup(Name = 'grp', CartId = wc.Id); insert g;
    insert new CartItem(Name = 'ci1', CartId = wc.Id, CartDeliveryGroupId = g.Id, Quantity = 3, SKU = 'sku1', Type = 'Product');
    return CartExtension.CartTestUtil.getCart(wc.Id);
  }
  @IsTest static void details() {
    String msg;
    try {
      CartExtension.Cart cart = mk();
      List<CartExtension.CartItemChange> changes = new List<CartExtension.CartItemChange>();
      changes.add(new CartExtension.CartItemChange.Builder().withChangedItem(CartExtension.OptionalCartItem.of(cart.getCartItems().get(0))).withAdded(true).build());
      CartExtension.BuyerActionDetails d = new CartExtension.BuyerActionDetails.Builder().withCartItemChanges(changes).build();
      msg = 'checkoutStarted=' + d.isCheckoutStarted() + ',changes=' + d.getCartItemChanges().size();
      CartExtension.OptionalBuyerActionDetails o = CartExtension.OptionalBuyerActionDetails.of(d);
      msg += ',present=' + o.isPresent() + ',orCheck=' + (!o.isPresent() || o.get().isCheckoutStarted());
      CartExtension.BuyerActionDetails e = new CartExtension.BuyerActionDetails.Builder().build();
      msg += ',emptyChanges=' + e.getCartItemChanges().size() + ',emptyCheckout=' + e.isCheckoutStarted();
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|details|' + msg);
  }
  @IsTest static void address() {
    String msg;
    try {
      CartExtension.Cart cart = mk();
      CartExtension.CartDeliveryGroup dg = cart.getCartDeliveryGroups().get(0);
      msg = 'before=' + (dg.getDeliverToAddress() == null);
      dg.setDeliverToStreet('newStreet'); dg.setDeliverToCity('newCity'); dg.setDeliverToState('Washington');
      dg.setDeliverToCountry('US'); dg.setDeliverToPostalCode('987654'); dg.setDeliverToLatitude(48.1); dg.setDeliverToLongitude(33.2);
      dg.setDeliverToGeocodeAccuracy(null);
      System.Address a = dg.getDeliverToAddress();
      msg += ',after=' + (a == null);
      if (a != null) msg += ',street=' + a.getStreet() + ',city=' + a.getCity() + ',state=' + a.getState() + ',country=' + a.getCountry() + ',postal=' + a.getPostalCode() + ',lat=' + a.getLatitude() + ',lon=' + a.getLongitude();
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|addr|' + msg);
  }
}
