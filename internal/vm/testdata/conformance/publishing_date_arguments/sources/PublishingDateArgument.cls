public class PublishingDateArgument {
    public static Map<String,Object> observe() {
        Map<String,Object> rows=new Map<String,Object>();
        for(Integer i=0;i<2;i++) {
            Knowledge__kav article=new Knowledge__kav(Title='Synthetic article '+i, UrlName='synthetic-article-'+i);
            insert article;
            article=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:article.Id];
            KbManagement.PublishingService.publishArticle(article.KnowledgeArticleId,true);
            Datetime scheduled=i==0 ? null : Datetime.now().addDays(1);
            try { KbManagement.PublishingService.archiveOnlineArticle(article.KnowledgeArticleId,scheduled); rows.put(i==0?'nullDate':'date','accepted'); }
            catch(Exception e) { rows.put(i==0?'nullDate':'date',e.getTypeName()+': '+e.getMessage()); }
        }
        return rows;
    }
}
