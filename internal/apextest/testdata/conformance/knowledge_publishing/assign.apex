String tag=String.valueOf(Datetime.now().getTime());
Knowledge__kav k=new Knowledge__kav(Title='Assign '+tag,UrlName='assign-'+tag,Language='en_US');insert k;k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId;
try { KbManagement.PublishingService.assignDraftArticleTask(parent,UserInfo.getUserId(),'Synthetic task',Datetime.now().addDays(1),true); System.debug('B4:assign_draft:returned'); } catch(Exception e) { System.debug('B4:assign_draft:'+e.getTypeName()); }
System.debug('B4:task_count:'+ [SELECT COUNT() FROM Task WHERE Subject='Synthetic task']);
KbManagement.PublishingService.publishArticle(parent,true);
try { KbManagement.PublishingService.assignDraftArticleTask(parent,UserInfo.getUserId(),'Synthetic task',Datetime.now().addDays(1),true); System.debug('B4:assign_no_draft:returned'); } catch(Exception e) { System.debug('B4:assign_no_draft:'+e.getTypeName()); }
