@IsTest private class B4CategoryProbe { @IsTest static void observe() {
 Knowledge__kav k=new Knowledge__kav(Title='Category Test',UrlName='category-test',Language='en_US'); insert k;
 insert new Knowledge__DataCategorySelection(ParentId=k.Id,DataCategoryGroupName='Guides',DataCategoryName='General');
 System.assertEquals(1,[SELECT COUNT() FROM Knowledge__DataCategorySelection WHERE ParentId=:k.Id]);
 System.assertEquals(1,[SELECT COUNT() FROM Knowledge__kav WHERE PublishStatus='Draft' AND Id=:k.Id WITH DATA CATEGORY Guides__c AT General__c]);
 try { insert new Knowledge__DataCategorySelection(ParentId=k.Id,DataCategoryGroupName='Guides__c',DataCategoryName='General__c'); System.assert(false); } catch(DmlException e) { System.assertEquals('INVALID_DATA_CATEGORY_GROUP_REFERENCE',String.valueOf(e.getDmlStatusCode(0))); }
} }