@IsTest
private class R5InvProbe {
  @IsTest static void sizes() {
    String msg;
    try {
      commerce_inventory.UpsertItemReservationRequest it = new commerce_inventory.UpsertItemReservationRequest(double.valueOf('10.0'), Id.valueOf('0ghSG0000000JFbYAM'), Id.valueOf('0a9SG000003AfY1YAK'), Id.valueOf('01tSG000001NNgKYAW'), null);
      List<commerce_inventory.UpsertItemReservationRequest> l = new List<commerce_inventory.UpsertItemReservationRequest>{ it };
      commerce_inventory.UpsertReservationRequest r = new commerce_inventory.UpsertReservationRequest(100, 'ident', '0a9SG000003AfY1YAK', l);
      msg = 'upsertItems=' + r.getItems().size() + ',ident=' + r.getReservationIdentifier() + ',secs=' + r.getReservationInSeconds() + ',src=' + r.getReservationSourceId() + ',qty=' + r.getItems().get(0).getQuantity() + ',assocNull=' + (r.getItems().get(0).getAssociatedProducts() == null);
      commerce_inventory.InventoryLevelsItemRequest ir = new commerce_inventory.InventoryLevelsItemRequest('01txx0000001aBcAAI', 'sku1', 'a1Bxx0000005T9E');
      Set<commerce_inventory.InventoryLevelsItemRequest> s = new Set<commerce_inventory.InventoryLevelsItemRequest>{ ir };
      commerce_inventory.InventoryLevelsRequest lr = new commerce_inventory.InventoryLevelsRequest('LocationGroup', s);
      msg += ',levels=' + lr.getItemInventoryLevelRequests().size() + ',type=' + lr.getInventoryLocationSourceType() + ',sku=' + lr.getItemInventoryLevelRequests().iterator().next().getStockKeepingUnit();
      commerce_inventory.InventoryLevelsRequest lr2 = new commerce_inventory.InventoryLevelsRequest(null, s);
      msg += ',levelsNullType=' + lr2.getItemInventoryLevelRequests().size() + ',typeNull=' + (lr2.getInventoryLocationSourceType() == null);
      commerce_inventory.UpsertReservationResponse resp = new commerce_inventory.UpsertReservationResponse();
      msg += ',respItemsNull=' + (resp.getItems() == null);
    } catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|inv|' + msg);
  }
}
