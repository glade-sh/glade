@IsTest private class B4CategoryBoundary { @IsTest static void observe() {
 Knowledge__kav k=new Knowledge__kav(Title='Boundary Test',UrlName='boundary-test',Language='en_US');insert k;insert new Knowledge__DataCategorySelection(ParentId=k.Id,DataCategoryGroupName='Guides',DataCategoryName='General');
 System.assertEquals(1,[SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c AT General__c]);
 System.assertEquals(1,[SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c ABOVE General__c]);
 System.assertEquals(1,[SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c BELOW General__c]);
 System.assertEquals(1,[SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c ABOVE_OR_BELOW General__c]);
 System.assertEquals(0,[SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c AT All__c]);
 System.assertEquals(0,[SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c ABOVE All__c]);
 System.assertEquals(1,[SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c BELOW All__c]);
 System.assertEquals(1,[SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c ABOVE_OR_BELOW All__c]);
} }