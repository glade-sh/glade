User target=[SELECT Id FROM User WHERE Alias='b4test' LIMIT 1];String tag=String.valueOf(Datetime.now().getTime());Knowledge__kav k=new Knowledge__kav(Title='Other '+tag,UrlName='other-'+tag,Language='en_US');insert k;
k=[SELECT KnowledgeArticleId,AssignedToId FROM Knowledge__kav WHERE Id=:k.Id];System.debug('B4:before_is_current:'+(k.AssignedToId==UserInfo.getUserId()));
KbManagement.PublishingService.assignDraftArticleTask(k.KnowledgeArticleId,target.Id,'Review',null,false);
k=[SELECT AssignedToId,AssignedById,AssignmentDate,AssignmentDueDate,AssignmentNote FROM Knowledge__kav WHERE Id=:k.Id];System.debug('B4:other:'+(k.AssignedToId==target.Id)+':'+(k.AssignedById==UserInfo.getUserId())+':'+(k.AssignmentDate!=null)+':'+(k.AssignmentDueDate==null)+':'+k.AssignmentNote);
