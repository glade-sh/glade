List<CartExtension.SalesItemTypeEnum> itemTypes = CartExtension.SalesItemTypeEnum.values();
System.debug('ENUM|' + itemTypes.size() + '|' + itemTypes[0] + '|' + itemTypes[1] + '|' + CartExtension.SalesItemTypeEnum.valueOf('PRODUCT').ordinal());
