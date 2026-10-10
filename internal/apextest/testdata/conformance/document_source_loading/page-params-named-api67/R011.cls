@IsTest private class CRT6732be0fd493ddR011 {

@IsTest static void R011() {
String corpusObserved;
try {
Object r;
Test.setCurrentPage(Page.ResPage67_f48cd82f84b648a89f0148cd); ApexPages.currentPage().getParameters().put('id','001000000000001AAA'); r=ApexPages.currentPage().getUrl();
corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|R011|' + corpusObserved);
}
}
