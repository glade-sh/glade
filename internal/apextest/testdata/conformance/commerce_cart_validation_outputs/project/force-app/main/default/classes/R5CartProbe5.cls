@IsTest
private class R5CartProbe5 {
  static Id cartId; static Id itemId;
  private static CartExtension.Cart mk(Boolean withCvo) {
    Account a = new Account(Name = 'acc'); insert a;
    WebStore ws = new WebStore(Name = 'ws', OptionsCartCalculateEnabled = true); insert ws;
    WebCart wc = new WebCart(Name = 'cart', WebStoreId = ws.Id, AccountId = a.Id, Status = 'Checkout'); insert wc;
    cartId = wc.Id;
    CartDeliveryGroup g = new CartDeliveryGroup(Name = 'grp', CartId = wc.Id); insert g;
    CartItem ci = new CartItem(Name = 'ci1', CartId = wc.Id, CartDeliveryGroupId = g.Id, Quantity = 3, SKU = 'sku1', Type = 'Product'); insert ci;
    itemId = ci.Id;
    if (withCvo) {
      insert new CartValidationOutput(CartId = wc.Id, Level = 'Error', Message = 'm1', Name = 'My Error CVO', RelatedEntityId = wc.Id, Type = 'Shipping');
      insert new CartValidationOutput(CartId = wc.Id, Level = 'Info', Message = 'm2', Name = 'cvo2', RelatedEntityId = ci.Id, Type = 'Pricing');
    }
    return CartExtension.CartTestUtil.getCart(wc.Id);
  }
  @IsTest static void loadedCvo() {
    String msg;
    try {
      CartExtension.Cart cart = mk(true);
      List<String> parts = new List<String>();
      Iterator<CartExtension.CartValidationOutput> it = cart.getCartValidationOutputs().iterator();
      while (it.hasNext()) {
        CartExtension.CartValidationOutput c = it.next();
        parts.add(c.getMessage() + ':type=' + c.getType() + ',level=' + c.getLevel() + ',msg=' + c.getMessage() + ',relCart=' + (c.getRelatedEntityId() == cartId) + ',relItem=' + (c.getRelatedEntityId() == itemId));
      }
      parts.sort();
      msg = 'size=' + cart.getCartValidationOutputs().size() + ' ## ' + String.join(parts, ' ## ');
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|cvo|' + msg);
  }
  @IsTest static void newCvo() {
    String msg;
    try {
      CartExtension.Cart cart = mk(false);
      CartExtension.CartItem item = cart.getCartItems().get(0);
      CartExtension.CartValidationOutput c = new CartExtension.CartValidationOutput(CartExtension.CartValidationOutputTypeEnum.PRICING, CartExtension.CartValidationOutputLevelEnum.ERROR, item);
      CartExtension.CartValidationOutput d = new CartExtension.CartValidationOutput(CartExtension.CartValidationOutputTypeEnum.PRICING, CartExtension.CartValidationOutputLevelEnum.ERROR);
      msg = 'itemIdEq=' + (item.getId() == itemId) + ',relItem=' + (c.getRelatedEntityId() == itemId) + ',relNull=' + (c.getRelatedEntityId() == null) + ',msg=' + c.getMessage() + ',type=' + c.getType() + ',level=' + c.getLevel() + ',dRelNull=' + (d.getRelatedEntityId() == null);
      c.setMessage('hello'); cart.getCartValidationOutputs().add(c);
      msg += ',size=' + cart.getCartValidationOutputs().size() + ',msg2=' + cart.getCartValidationOutputs().get(0).getMessage();
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|new|' + msg);
  }
  @IsTest static void change() {
    String msg;
    try {
      CartExtension.Cart cart = mk(false);
      CartExtension.CartItem item = cart.getCartItems().get(0);
      CartExtension.CartItemChange ch = new CartExtension.CartItemChange.Builder().withChangedItem(CartExtension.OptionalCartItem.of(item)).withAdded(true).build();
      CartExtension.OptionalCartItem oi = ch.getChangedItem();
      msg = 'oiNull=' + (oi == null);
      if (oi != null) msg += ',present=' + oi.isPresent() + ',sku=' + oi.get().getSku() + ',added=' + ch.isAdded();
      CartExtension.CartItemChange e2 = new CartExtension.CartItemChange.Builder().build();
      msg += ',emptyOiNull=' + (e2.getChangedItem() == null) + ',emptyAdded=' + e2.isAdded();
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|change|' + msg);
  }
}
