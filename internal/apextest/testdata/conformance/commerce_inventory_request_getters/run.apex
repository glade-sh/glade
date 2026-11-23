List<String> out = new List<String>();
Account a = new Account(Name = 'acc'); insert a;
try { new commerce_inventory.UpsertItemReservationRequest(double.valueOf('10.0'), a.Id, a.Id, a.Id, null); out.add('item=ok'); } catch (Exception e) { out.add('item=' + e.getTypeName() + '|' + e.getMessage()); }
try { new commerce_inventory.UpsertReservationRequest(100, 'ident', String.valueOf(a.Id), new List<commerce_inventory.UpsertItemReservationRequest>()); out.add('upsert=ok'); } catch (Exception e) { out.add('upsert=' + e.getTypeName() + '|' + e.getMessage()); }
try { new commerce_inventory.InventoryLevelsItemRequest(a.Id, 'sku1', a.Id); out.add('lvlItem=ok'); } catch (Exception e) { out.add('lvlItem=' + e.getTypeName() + '|' + e.getMessage()); }
try { new commerce_inventory.InventoryLevelsRequest('LocationGroup', new Set<commerce_inventory.InventoryLevelsItemRequest>()); out.add('lvl=ok'); } catch (Exception e) { out.add('lvl=' + e.getTypeName() + '|' + e.getMessage()); }
try { new commerce_inventory.UpsertReservationResponse(); out.add('resp=ok'); } catch (Exception e) { out.add('resp=' + e.getTypeName() + '|' + e.getMessage()); }
try { new CartExtension.CartDeliveryGroupMethod('n', 1.0, a.Id); out.add('cdgm=ok'); } catch (Exception e) { out.add('cdgm=' + e.getTypeName() + '|' + e.getMessage()); }
try { CartExtension.CartTestUtil.createCart(); out.add('create=ok'); } catch (Exception e) { out.add('create=' + e.getTypeName() + '|' + e.getMessage()); }
System.debug('R5|inv|' + String.join(out, ' ## '));
