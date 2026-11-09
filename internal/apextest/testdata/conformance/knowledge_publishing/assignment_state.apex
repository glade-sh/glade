String tag=String.valueOf(Datetime.now().getTime()); Knowledge__kav k=new Knowledge__kav(Title='Assignment '+tag,UrlName='assignment-'+tag,Language='en_US');insert k;
k=[SELECT KnowledgeArticleId,AssignedToId,AssignedById,AssignmentDate,AssignmentDueDate,AssignmentNote FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId;
System.debug('B4:before:'+(k.AssignedToId==null)+':'+(k.AssignmentNote==null)+':'+(k.AssignmentDueDate==null));
Datetime due=Datetime.now().addDays(1); KbManagement.PublishingService.assignDraftArticleTask(parent,UserInfo.getUserId(),'Synthetic task',due,true);
k=[SELECT AssignedToId,AssignedById,AssignmentDate,AssignmentDueDate,AssignmentNote FROM Knowledge__kav WHERE Id=:k.Id];
System.debug('B4:after:'+(k.AssignedToId==UserInfo.getUserId())+':'+(k.AssignedById==UserInfo.getUserId())+':'+(k.AssignmentDate!=null)+':'+(k.AssignmentDueDate==due)+':'+k.AssignmentNote);
System.debug('B4:due_values:'+String.valueOf(due)+':'+String.valueOf(k.AssignmentDueDate)+':'+k.AssignmentDueDate.getTime()+':'+due.getTime());
