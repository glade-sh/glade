@IsTest private class PgRef {
 static String jsonOf(Object o) { try { return JSON.serialize(o); } catch (Exception e) { return 'ex:' + e.getTypeName() + ':' + e.getMessage(); } }
 @IsTest static void t() {
  String p = 'pg';
  PageReference a = new PageReference('/apex/psrpage'); PageReference g = Page.PsrPage; PageReference x = new PageReference('https://example.invalid/a?b=c');
  p += '|valueOf=' + String.valueOf(a) + '|toString=' + a.toString() + '|concat=' + ('' + a) + '|getUrl=' + a.getUrl();
  p += '|none=0';
  p += '|pageValueOf=' + String.valueOf(g) + '|pageToString=' + g.toString();
  p += '|extValueOf=' + String.valueOf(x) + '|extToString=' + x.toString();
  p += '|listValueOf=' + String.valueOf(new List<PageReference>{a}) + '|jsonSerialize=' + jsonOf(a);
  p += '|currentBefore=' + String.valueOf(ApexPages.currentPage());
  Test.setCurrentPage(a); p += '|currentAfter=' + String.valueOf(ApexPages.currentPage()) + '|currentToString=' + ApexPages.currentPage().toString();
  p += '|format=' + String.format('{0}', new List<Object>{a}) + '|debug=' + a;
  System.assert(false, p);
 }
}
