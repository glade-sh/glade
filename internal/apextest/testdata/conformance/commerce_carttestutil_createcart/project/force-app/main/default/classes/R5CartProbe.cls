@IsTest
private class R5CartProbe {
  private static String facts(CartExtension.Cart cart) {
    List<String> out = new List<String>();
    out.add('name=' + cart.getName());
    out.add('status=' + cart.getStatus());
    out.add('currency=' + cart.getCurrencyIsoCode());
    out.add('accountNull=' + (cart.getAccountId() == null));
    out.add('webStoreNull=' + (cart.getWebStoreId() == null));
    out.add('idNull=' + (cart.getId() == null));
    out.add('items=' + cart.getCartItems().size());
    out.add('groups=' + cart.getCartDeliveryGroups().size());
    if (cart.getCartDeliveryGroups().size() > 0) {
      CartExtension.CartDeliveryGroup g = cart.getCartDeliveryGroups().get(0);
      out.add('gName=' + g.getName());
      out.add('gIdNull=' + (g.getId() == null));
      out.add('gAddrNull=' + (g.getDeliverToAddress() == null));
    }
    out.add('validation=' + cart.getCartValidationOutputs().size());
    out.add('db:cart=' + [SELECT COUNT() FROM WebCart] + ',acct=' + [SELECT COUNT() FROM Account] + ',store=' + [SELECT COUNT() FROM WebStore] + ',group=' + [SELECT COUNT() FROM CartDeliveryGroup] + ',item=' + [SELECT COUNT() FROM CartItem]);
    return String.join(out, ';');
  }
  @IsTest static void noArg() {
    String msg;
    try { msg = facts(CartExtension.CartTestUtil.createCart()); } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|noArg|' + msg);
  }
  @IsTest static void b2ce() {
    String msg;
    try { msg = facts(CartExtension.CartTestUtil.createCart(CartExtension.WebStoreTypeEnum.B2CE)); } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|b2ce|' + msg);
  }
  @IsTest static void b2b() {
    String msg;
    try { msg = facts(CartExtension.CartTestUtil.createCart(CartExtension.WebStoreTypeEnum.B2B)); } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|b2b|' + msg);
  }
}
