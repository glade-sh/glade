String tag=String.valueOf(Datetime.now().getTime());Knowledge__kav k=new Knowledge__kav(Title='Categories '+tag,UrlName='categories-'+tag,Language='en_US');insert k;
insert new Knowledge__DataCategorySelection(ParentId=k.Id,DataCategoryGroupName='Guides',DataCategoryName='General');
System.debug('B4:cat_at:'+ [SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c AT General__c]);
System.debug('B4:cat_above:'+ [SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c ABOVE General__c]);
System.debug('B4:cat_below:'+ [SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c BELOW General__c]);
System.debug('B4:cat_both:'+ [SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c ABOVE_OR_BELOW General__c]);

System.debug('B4:root_at:'+ [SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c AT All__c]);
System.debug('B4:root_above:'+ [SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c ABOVE All__c]);
System.debug('B4:root_below:'+ [SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c BELOW All__c]);
System.debug('B4:root_both:'+ [SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id WITH DATA CATEGORY Guides__c ABOVE_OR_BELOW All__c]);
