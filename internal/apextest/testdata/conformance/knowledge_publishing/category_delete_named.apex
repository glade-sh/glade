@IsTest private class B4CategoryDelete62 { @IsTest static void observe() {
 Knowledge__kav article=new Knowledge__kav(Title='Synthetic category deletion',UrlName='synthetic-category-deletion',Language='en_US'); insert article;
 Knowledge__DataCategorySelection sel=new Knowledge__DataCategorySelection(ParentId=article.Id,DataCategoryGroupName='Guides',DataCategoryName='General'); insert sel;
 System.assertEquals(1,[SELECT COUNT() FROM Knowledge__kav WHERE PublishStatus='Draft' AND Id=:article.Id WITH DATA CATEGORY Guides__c AT General__c]);
 delete sel;
 System.assertEquals(0,[SELECT COUNT() FROM Knowledge__DataCategorySelection WHERE Id=:sel.Id ALL ROWS]);
 System.assertEquals(0,[SELECT COUNT() FROM Knowledge__kav WHERE PublishStatus='Draft' AND Id=:article.Id WITH DATA CATEGORY Guides__c AT General__c]);
} }
