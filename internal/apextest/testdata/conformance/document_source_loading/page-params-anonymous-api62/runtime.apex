class P {
  public void out(String id, Object v) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
try { Object r; Test.setCurrentPage(Page.ResPage62_f48cd82f84b648a89f0148cd); ApexPages.currentPage().getParameters().put('id','001000000000001AAA'); r=ApexPages.currentPage().getUrl(); pq.out('R011', r); } catch (Exception e) { pq.err('R011', e); }
try { Object r; Test.setCurrentPage(new PageReference('/apex/ResPage62_f48cd82f84b648a89f0148cd')); ApexPages.currentPage().getParameters().put('id','001000000000001AAA'); r=ApexPages.currentPage().getUrl(); pq.out('R012', r); } catch (Exception e) { pq.err('R012', e); }
