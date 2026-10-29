public class ObjectAvailabilityChildren {
 public static Map<String,Object> observe() {
  Map<String,Object> r=new Map<String,Object>();
  try { Database.query('SELECT Id,(SELECT Id FROM quotes) FROM opportunity'); r.put('quotesLower',true); } catch (Exception e) { r.put('quotesLower',e.getTypeName()); }
  try { Database.query('SELECT Id,(SELECT Id FROM Quotes) FROM Account'); r.put('accountQuotes',true); } catch (Exception e) { r.put('accountQuotes',e.getTypeName()); }
  try { Database.query('SELECT Id,(SELECT Id FROM quotelineitems) FROM quote'); r.put('linesLower',true); } catch (Exception e) { r.put('linesLower',e.getTypeName()); }
  try { Database.query('SELECT Id,(SELECT Id FROM Quotes) FROM Opportunity'); r.put('quotes',true); } catch (Exception e) { r.put('quotes',e.getTypeName()); }
  try { Database.query('SELECT Id,(SELECT Id FROM QuoteLineItems) FROM Quote'); r.put('lines',true); } catch (Exception e) { r.put('lines',e.getTypeName()); }
  try { Database.query('SELECT Id,(SELECT Id FROM Contacts) FROM Account'); r.put('boundary',true); } catch (Exception e) { r.put('boundary',e.getTypeName()); }
  try { Database.query('SELECT Id,(SELECT Id FROM Quote) FROM Account'); r.put('invalidQuote',true); } catch (Exception e) { r.put('invalidQuote',e.getTypeName()); }
  try { Database.query('SELECT Id,(SELECT Id FROM WebStore) FROM Account'); r.put('invalidWebStore',true); } catch (Exception e) { r.put('invalidWebStore',e.getTypeName()); }
  return r;
 }
}
