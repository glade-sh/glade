@IsTest private class CRT6732be0fd493ddC001 {
@IsTest static void C001() {
PageReference missing=Page.ResMissing67_f48cd82f84b648a89f0148cd;
Test.setCurrentPage(missing);
System.debug(ApexPages.currentPage().getUrl());
}
}
