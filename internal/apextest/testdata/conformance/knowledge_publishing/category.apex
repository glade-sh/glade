String tag=String.valueOf(Datetime.now().getTime()); Knowledge__kav k=new Knowledge__kav(Title='Category '+tag,UrlName='category-'+tag,Language='en_US'); insert k;
Knowledge__DataCategorySelection sel=new Knowledge__DataCategorySelection(ParentId=k.Id,DataCategoryGroupName='Guides',DataCategoryName='General');
try { insert sel; System.debug('B4:category_insert:returned'); } catch(Exception e) { System.debug('B4:category_insert:'+e.getTypeName()+':'+e.getMessage()); }
System.debug('B4:category_count:'+ [SELECT COUNT() FROM Knowledge__DataCategorySelection WHERE ParentId=:k.Id]);
List<Knowledge__kav> selected=[SELECT Id FROM Knowledge__kav WHERE PublishStatus='Draft' AND Id=:k.Id WITH DATA CATEGORY Guides__c AT General__c];
System.debug('B4:category_query:'+selected.size());
try { insert new Knowledge__DataCategorySelection(ParentId=k.Id,DataCategoryGroupName='Guides__c',DataCategoryName='General__c'); System.debug('B4:category_invalid:returned'); } catch(DmlException e) { System.debug('B4:category_invalid:'+e.getDmlStatusCode(0)); }
