@IsTest private class CRT62bf8290a041c2C001 {
@IsTest static void C001() {
PageReference missing=Page.ResMissing62_f48cd82f84b648a89f0148cd;
Test.setCurrentPage(missing);
System.debug(ApexPages.currentPage().getUrl());
}
}
