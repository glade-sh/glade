public class R5Inv {
  public static String run() {
    String msg;
    try {
      Account a = new Account(Name = 'acc'); insert a;
      Product2 p = new Product2(Name = 'p', IsActive = true); insert p;
      commerce_inventory.UpsertItemReservationRequest it = new commerce_inventory.UpsertItemReservationRequest(double.valueOf('10.0'), a.Id, a.Id, p.Id, null);
      List<commerce_inventory.UpsertItemReservationRequest> l = new List<commerce_inventory.UpsertItemReservationRequest>{ it };
      commerce_inventory.UpsertReservationRequest r = new commerce_inventory.UpsertReservationRequest(100, 'ident', String.valueOf(a.Id), l);
      msg = 'upsertItems=' + r.getItems().size() + ',ident=' + r.getReservationIdentifier() + ',secs=' + r.getReservationInSeconds() + ',srcOk=' + (r.getReservationSourceId() == a.Id) + ',qty=' + r.getItems().get(0).getQuantity() + ',prodOk=' + (r.getItems().get(0).getProductId() == p.Id) + ',assocNull=' + (r.getItems().get(0).getAssociatedProducts() == null);
      commerce_inventory.InventoryLevelsItemRequest ir = new commerce_inventory.InventoryLevelsItemRequest(p.Id, 'sku1', a.Id);
      Set<commerce_inventory.InventoryLevelsItemRequest> s = new Set<commerce_inventory.InventoryLevelsItemRequest>{ ir };
      commerce_inventory.InventoryLevelsRequest lr = new commerce_inventory.InventoryLevelsRequest('LocationGroup', s);
      msg += ',levels=' + lr.getItemInventoryLevelRequests().size() + ',type=' + lr.getInventoryLocationSourceType() + ',sku=' + lr.getItemInventoryLevelRequests().iterator().next().getStockKeepingUnit();
      commerce_inventory.InventoryLevelsRequest lr2 = new commerce_inventory.InventoryLevelsRequest(null, s);
      msg += ',levelsNullType=' + lr2.getItemInventoryLevelRequests().size() + ',typeNull=' + (lr2.getInventoryLocationSourceType() == null);
      commerce_inventory.UpsertReservationResponse resp = new commerce_inventory.UpsertReservationResponse();
      msg += ',respItemsNull=' + (resp.getItems() == null);
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    return msg;
  }
}
