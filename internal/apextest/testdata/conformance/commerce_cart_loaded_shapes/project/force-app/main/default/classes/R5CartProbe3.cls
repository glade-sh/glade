@IsTest
private class R5CartProbe3 {
  private static Id arrange(String status) {
    Account a = new Account(Name = 'acc'); insert a;
    WebStore ws = new WebStore(Name = 'ws', OptionsCartCalculateEnabled = true); insert ws;
    WebCart wc = new WebCart(Name = 'cart', WebStoreId = ws.Id, AccountId = a.Id, Status = status); insert wc;
    return wc.Id;
  }
  private static void items(Id cartId, Boolean withGroup) {
    CartDeliveryGroup g = new CartDeliveryGroup(Name = 'grp', CartId = cartId); insert g;
    for (Integer i = 1; i <= 2; i++) {
      insert new CartItem(Name = 'ci' + i, CartId = cartId, CartDeliveryGroupId = g.Id, Quantity = 3, SKU = 'sku' + i, Type = 'Product');
    }
  }
  @IsTest static void loaded() {
    String msg;
    try {
      Id cartId = arrange('Checkout'); items(cartId, true);
      CartExtension.Cart cart = CartExtension.CartTestUtil.getCart(cartId);
      List<String> parts = new List<String>();
      Iterator<CartExtension.CartItem> it = cart.getCartItems().iterator();
      while (it.hasNext()) {
        CartExtension.CartItem ci = it.next();
        Decimal p = 2.5;
        parts.add(ci.getSku() + ':qty=' + ci.getQuantity() + ',list=' + ci.getListPrice() + ',total=' + ci.getTotalPrice() + ',type=' + ci.getType() + ',times=' + (p * ci.getQuantity()));
      }
      parts.sort();
      CartExtension.CartDeliveryGroup dg = cart.getCartDeliveryGroups().get(0);
      parts.add('dgName=' + dg.getName() + ',addrNull=' + (dg.getDeliverToAddress() == null) + ',methods=' + dg.getCartDeliveryGroupMethods().size() + ',selNull=' + (dg.getSelectedCartDeliveryGroupMethod() == null));
      parts.add('status=' + cart.getStatus() + ',items=' + cart.getCartItems().size() + ',groups=' + cart.getCartDeliveryGroups().size() + ',cvo=' + cart.getCartValidationOutputs().size());
      msg = String.join(parts, ' ## ');
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|cart|' + msg);
  }
  @IsTest static void groupAddress() {
    String msg;
    try {
      Id cartId = arrange('Checkout'); items(cartId, true);
      CartExtension.Cart cart = CartExtension.CartTestUtil.getCart(cartId);
      CartExtension.CartDeliveryGroup dg = cart.getCartDeliveryGroups().get(0);
      CartExtension.CartDeliveryGroupMethod m = new CartExtension.CartDeliveryGroupMethod('Ground', 10.99, null);
      msg = 'methodFee=' + m.getShippingFee() + ',name=' + m.getName();
      dg.getCartDeliveryGroupMethods().add(m);
      dg.setSelectedCartDeliveryGroupMethod(m);
      msg += ',sel=' + dg.getSelectedCartDeliveryGroupMethod().getName() + ',selFee=' + dg.getSelectedCartDeliveryGroupMethod().getShippingFee();
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|grp|' + msg);
  }
  public class P extends CartExtension.PricingCartCalculator {
    public virtual override void calculate(CartExtension.CartCalculateCalculatorRequest request) { super.calculate(request); }
  }
  public class I extends CartExtension.InventoryCartCalculator {
    public virtual override void calculate(CartExtension.CartCalculateCalculatorRequest request) { super.calculate(request); }
  }
  @IsTest static void superCalc() {
    String msg = '';
    for (Integer k = 0; k < 2; k++) {
      try {
        Id cartId = arrange('Checkout'); items(cartId, true);
        CartExtension.Cart cart = CartExtension.CartTestUtil.getCart(cartId);
        CartExtension.CartCalculateCalculatorRequest req = new CartExtension.CartCalculateCalculatorRequest(cart, CartExtension.OptionalBuyerActionDetails.empty());
        if (k == 0) new P().calculate(req); else new I().calculate(req);
        msg += (k == 0 ? 'pricing' : 'inventory') + '=ok,cvo=' + cart.getCartValidationOutputs().size() + ';';
      } catch (Exception e) { msg += (k == 0 ? 'pricing' : 'inventory') + '=' + e.getTypeName() + '|' + e.getMessage() + ';'; }
    }
    System.assert(false, 'R5|super|' + msg);
  }
}
