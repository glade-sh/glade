@IsTest
private class R5CartProbe6 {
  @IsTest static void methods() {
    Account a = new Account(Name = 'acc'); insert a;
    WebStore ws = new WebStore(Name = 'ws', OptionsCartCalculateEnabled = true); insert ws;
    WebCart wc = new WebCart(Name = 'cart', WebStoreId = ws.Id, AccountId = a.Id, Status = 'Checkout'); insert wc;
    CartDeliveryGroup g = new CartDeliveryGroup(Name = 'grp', CartId = wc.Id); insert g;
    insert new CartItem(Name = 'ci1', CartId = wc.Id, CartDeliveryGroupId = g.Id, Quantity = 3, SKU = 'sku1', Type = 'Product');
    CartExtension.Cart cart = CartExtension.CartTestUtil.getCart(wc.Id);
    CartExtension.CartDeliveryGroup dg = cart.getCartDeliveryGroups().get(0);
    CartExtension.CartDeliveryGroupMethod m1 = new CartExtension.CartDeliveryGroupMethod('Ground', 10.99, a.Id);
    CartExtension.CartDeliveryGroupMethod m2 = new CartExtension.CartDeliveryGroupMethod('Fast', 15.99, a.Id);
    dg.getCartDeliveryGroupMethods().add(m1);
    dg.getCartDeliveryGroupMethods().add(m2);
    String s = 'size=' + dg.getCartDeliveryGroupMethods().size();
    Iterator<CartExtension.CartDeliveryGroupMethod> it = dg.getCartDeliveryGroupMethods().iterator();
    Decimal minFee = null; CartExtension.CartDeliveryGroupMethod best = null;
    while (it.hasNext()) { CartExtension.CartDeliveryGroupMethod x = it.next(); if (minFee == null || x.getShippingFee() < minFee) { minFee = x.getShippingFee(); best = x; } }
    dg.setSelectedCartDeliveryGroupMethod(best);
    CartExtension.CartDeliveryGroupMethod sel = cart.getCartDeliveryGroups().get(0).getSelectedCartDeliveryGroupMethod();
    s += ',selNull=' + (sel == null);
    if (sel != null) s += ',selName=' + sel.getName() + ',fee=' + sel.getShippingFee();
    dg.getCartDeliveryGroupMethods().clear();
    s += ',afterClear=' + dg.getCartDeliveryGroupMethods().size();
    System.assert(false, 'R5|meth|' + s);
  }
  @IsTest static void selectedAfterClear() {
    Account a = new Account(Name = 'acc'); insert a;
    WebStore ws = new WebStore(Name = 'ws', OptionsCartCalculateEnabled = true); insert ws;
    WebCart wc = new WebCart(Name = 'cart', WebStoreId = ws.Id, AccountId = a.Id, Status = 'Checkout'); insert wc;
    CartDeliveryGroup g = new CartDeliveryGroup(Name = 'grp', CartId = wc.Id); insert g;
    CartExtension.Cart cart = CartExtension.CartTestUtil.getCart(wc.Id);
    CartExtension.CartDeliveryGroup dg = cart.getCartDeliveryGroups().get(0);
    CartExtension.CartDeliveryGroupMethod m1 = new CartExtension.CartDeliveryGroupMethod('Ground', 10.99, a.Id);
    dg.getCartDeliveryGroupMethods().add(m1);
    dg.setSelectedCartDeliveryGroupMethod(m1);
    dg.getCartDeliveryGroupMethods().clear();
    CartExtension.CartDeliveryGroupMethod sel = dg.getSelectedCartDeliveryGroupMethod();
    String s = 'afterClearSelNull=' + (sel == null);
    CartExtension.CartDeliveryGroupMethod other = new CartExtension.CartDeliveryGroupMethod('Fast', 15.99, a.Id);
    dg.setSelectedCartDeliveryGroupMethod(other);
    s += ',notInListSelNull=' + (dg.getSelectedCartDeliveryGroupMethod() == null);
    dg.setSelectedCartDeliveryGroupMethod(null);
    s += ',nullSetSelNull=' + (dg.getSelectedCartDeliveryGroupMethod() == null);
    System.assert(false, 'R5|clr|' + s);
  }
}
