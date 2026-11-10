public with sharing class PageErrorHostV67 {
 @AuraEnabled public static String html() { try { Blob b=new PageReference('/apex/MissingHostPage').getContent(); return 'rendered'; } catch(Exception e) {return 'caught|'+e.getTypeName()+'|'+e.getMessage();} }
 @AuraEnabled public static String pdf() { try { Blob b=new PageReference('/apex/MissingHostPage').getContentAsPDF(); return 'rendered'; } catch(Exception e) {return 'caught|'+e.getTypeName()+'|'+e.getMessage();} }
 public String getHtmlResult() { return html(); }
 public String getPdfResult() { return pdf(); }
}