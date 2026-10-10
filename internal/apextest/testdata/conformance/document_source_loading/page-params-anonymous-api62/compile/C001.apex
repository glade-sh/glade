class P {
  public void out(String id, Object v) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
PageReference missing=Page.ResMissing62_f48cd82f84b648a89f0148cd; Test.setCurrentPage(missing); r=ApexPages.currentPage().getUrl();
