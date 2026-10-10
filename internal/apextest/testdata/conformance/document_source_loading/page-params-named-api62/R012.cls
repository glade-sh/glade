@IsTest private class CRT62bf8290a041c2R012 {

@IsTest static void R012() {
String corpusObserved;
try {
Object r;
Test.setCurrentPage(new PageReference('/apex/ResPage62_f48cd82f84b648a89f0148cd')); ApexPages.currentPage().getParameters().put('id','001000000000001AAA'); r=ApexPages.currentPage().getUrl();
corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|R012|' + corpusObserved);
}
}
