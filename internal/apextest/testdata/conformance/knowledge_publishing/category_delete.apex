String tag=String.valueOf(Datetime.now().getTime()); Knowledge__kav article=new Knowledge__kav(Title='Category deletion '+tag,UrlName='category-deletion-'+tag,Language='en_US'); insert article;
Knowledge__DataCategorySelection sel=new Knowledge__DataCategorySelection(ParentId=article.Id,DataCategoryGroupName='Guides',DataCategoryName='General'); insert sel;
System.debug('B4:category_before:'+ [SELECT COUNT() FROM Knowledge__kav WHERE PublishStatus='Draft' AND Id=:article.Id WITH DATA CATEGORY Guides__c AT General__c]);
try { delete sel; System.debug('B4:category_delete:returned'); } catch(Exception e) { System.debug('B4:category_delete:'+e.getTypeName()); }
System.debug('B4:category_after:'+ [SELECT COUNT() FROM Knowledge__kav WHERE PublishStatus='Draft' AND Id=:article.Id WITH DATA CATEGORY Guides__c AT General__c]);
