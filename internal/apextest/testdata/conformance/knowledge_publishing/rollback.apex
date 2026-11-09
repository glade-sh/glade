Savepoint p=Database.setSavepoint(); String tag=String.valueOf(Datetime.now().getTime());
Knowledge__kav k=new Knowledge__kav(Title='Rollback '+tag,UrlName='rollback-'+tag,Language='en_US'); insert k; k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId; Database.rollback(p);
try { KbManagement.PublishingService.archiveOnlineArticle(String.valueOf(parent),null); System.debug('B4:missing_valid_archive:returned'); } catch(Exception e) { System.debug('B4:missing_valid_archive:'+e.getTypeName()); }
try { KbManagement.PublishingService.editOnlineArticle(parent,false); System.debug('B4:missing_valid_edit:returned'); } catch(Exception e) { System.debug('B4:missing_valid_edit:'+e.getTypeName()); }
