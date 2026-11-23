List<String> out = new List<String>();
try { CartExtension.CartTestUtil.getCart('0aB000000000001AAA'); out.add('getCart=ok'); } catch (Exception e) { out.add('getCart=' + e.getTypeName() + '|' + e.getMessage()); }
try { CartExtension.CartTestUtil.createCart(CartExtension.WebStoreTypeEnum.B2B); out.add('createB2B=ok'); } catch (Exception e) { out.add('createB2B=' + e.getTypeName() + '|' + e.getMessage()); }
try { CartExtension.CartTestUtil.createCart(CartExtension.WebStoreTypeEnum.B2CE); out.add('createB2CE=ok'); } catch (Exception e) { out.add('createB2CE=' + e.getTypeName() + '|' + e.getMessage()); }
try { CartExtension.CartTestUtil.createCart(); out.add('create=ok'); } catch (Exception e) { out.add('create=' + e.getTypeName() + '|' + e.getMessage()); }
System.debug('R5|cu|' + String.join(out, ' ## '));
