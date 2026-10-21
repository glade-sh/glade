@IsTest private class CartSliceTest {
 @IsTest static void cart_fixture() {
  Map<String,Object> observed=new Map<String,Object>();
  try {
   CartExtension.Cart cart=CartExtension.CartTestUtil.createCart(CartExtension.WebStoreTypeEnum.B2CE);
   Id identity=cart.getId();
   observed.put('hasId',identity!=null);
   observed.put('stored',[SELECT Count() FROM WebCart WHERE Id=:identity WITH SYSTEM_MODE]);
   WebCart stored=[SELECT Name,Status,AccountId,WebStoreId,CurrencyIsoCode FROM WebCart WHERE Id=:identity WITH SYSTEM_MODE];
   observed.put('name',stored.Name);observed.put('status',stored.Status);observed.put('currency',stored.CurrencyIsoCode);observed.put('hasAccount',stored.AccountId!=null);observed.put('hasStore',stored.WebStoreId!=null);
   observed.put('accountName',[SELECT Name FROM Account WHERE Id=:stored.AccountId WITH SYSTEM_MODE].Name);
   observed.put('storeName',[SELECT Name FROM WebStore WHERE Id=:stored.WebStoreId WITH SYSTEM_MODE].Name);
   observed.put('loadedIdentity',CartExtension.CartTestUtil.getCart(identity).getId()==identity);
  } catch(Exception e){observed.put('exception',e.getTypeName());observed.put('message',e.getMessage());}
  System.assert(false,'OBSERVED|cart_fixture|'+JSON.serialize(observed));
 }
 @IsTest static void distinct_carts() {
  CartExtension.Cart first=CartExtension.CartTestUtil.createCart(CartExtension.WebStoreTypeEnum.B2CE);
  CartExtension.Cart second=CartExtension.CartTestUtil.createCart(CartExtension.WebStoreTypeEnum.B2CE);
  Set<Id> identities=new Set<Id>{first.getId(),second.getId()};
  WebCart one=[SELECT AccountId,WebStoreId FROM WebCart WHERE Id=:first.getId() WITH SYSTEM_MODE];
  WebCart two=[SELECT AccountId,WebStoreId FROM WebCart WHERE Id=:second.getId() WITH SYSTEM_MODE];
  System.assert(false,'OBSERVED|distinct_carts|'+JSON.serialize(new Map<String,Object>{'sameAccount'=>one.AccountId==two.AccountId,'sameStore'=>one.WebStoreId==two.WebStoreId,'distinct'=>identities.size()==2,'stored'=>[SELECT Count() FROM WebCart WHERE Id IN :identities WITH SYSTEM_MODE]}));
 }
}
