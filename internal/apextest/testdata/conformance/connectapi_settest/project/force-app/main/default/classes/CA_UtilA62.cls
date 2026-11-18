public class CA_UtilA62 {
    public static ConnectApi.FeedElementPage page(String marker) {
        ConnectApi.FeedElementPage p = new ConnectApi.FeedElementPage();
        p.currentPageUrl = marker;
        p.elements = new List<ConnectApi.FeedElement>();
        return p;
    }
    public static ConnectApi.KnowledgeArticleVersionCollection articles(String marker) {
        ConnectApi.KnowledgeArticleVersionCollection c = new ConnectApi.KnowledgeArticleVersionCollection();
        c.items = new List<ConnectApi.KnowledgeArticleVersion>();
        return c;
    }
    public static String ex(Exception e) {
        return e.getTypeName() + ' | ' + e.getMessage();
    }
}
