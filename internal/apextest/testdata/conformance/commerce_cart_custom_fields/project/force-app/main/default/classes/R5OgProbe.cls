@IsTest
private class R5OgProbe {
  private static CartExtension.Cart mk() {
    Account a = new Account(Name = 'acc'); insert a;
    WebStore ws = new WebStore(Name = 'ws'); insert ws;
    WebCart wc = new WebCart(Name = 'cart', WebStoreId = ws.Id, AccountId = a.Id); insert wc;
    CartDeliveryGroup g = new CartDeliveryGroup(Name = 'grp', CartId = wc.Id, Cf__c = 'gv'); insert g;
    insert new CartItem(Name = 'ci', SalesPrice = 10, CartId = wc.Id, CartDeliveryGroupId = g.Id, Type = 'Product', Cf__c = 'iv');
    return CartExtension.CartTestUtil.getCart(wc.Id);
  }
  private static String step(CartExtension.Cart cart, String what) {
    try {
      CartExtension.CartDeliveryGroup dg = cart.getCartDeliveryGroups().get(0);
      CartExtension.CartItem ci = cart.getCartItems().get(0);
      if (what == 'dgGet') return String.valueOf(dg.getCustomField('Cf__c'));
      if (what == 'dgSet') { dg.setCustomField('Cf__c', 'g2'); return String.valueOf(dg.getCustomField('Cf__c')); }
      if (what == 'ciGet') return String.valueOf(ci.getCustomField('Cf__c'));
      if (what == 'ciSet') { ci.setCustomField('Cf__c', 'i2'); return String.valueOf(ci.getCustomField('Cf__c')); }
      if (what == 'nope') return String.valueOf(dg.getCustomField('Nope__c'));
      if (what == 'std') return String.valueOf(dg.getCustomField('Name'));
      if (what == 'nullName') return String.valueOf(dg.getCustomField(null));
      if (what == 'setStd') { dg.setCustomField('Name', 'x'); return 'ok'; }
      return 'unknown';
    } catch (Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
  }
  @IsTest static void custom() {
    CartExtension.Cart cart = mk();
    List<String> out = new List<String>();
    for (String w : new List<String>{'dgGet','dgSet','ciGet','ciSet','nope','std','nullName','setStd'}) out.add(w + '=' + step(cart, w));
    System.assert(false, 'R5|cf|' + String.join(out, ' ## '));
  }
}
