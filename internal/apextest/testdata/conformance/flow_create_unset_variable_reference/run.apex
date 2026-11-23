String msg = '';
try {
  Product2 p = new Product2(Name='pb-probe', IsActive=true);
  insert p;
  msg = 'pricebooks=' + [SELECT COUNT() FROM Pricebook2] + ',entries=' + [SELECT COUNT() FROM PricebookEntry WHERE Product2Id = :p.Id];
} catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
System.debug('R5|pb|' + msg);
