@IsTest
private class CA_ScopeA67 {
    @TestSetup
    static void setup() {
        ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'me', CA_UtilA67.page('SETUP'));
    }
    @IsTest
    static void c5a_setup_visible() {
        String obs = 'C5a';
        try {
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'me');
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c5b_before_starttest() {
        String obs = 'C5b';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P5b'));
            Test.startTest();
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            Test.stopTest();
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c5c_inside_starttest() {
        String obs = 'C5c';
        try {
            String uid = UserInfo.getUserId();
            Test.startTest();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 26, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P5c'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 26, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            Test.stopTest();
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c5d_after_stoptest() {
        String obs = 'C5d';
        try {
            String uid = UserInfo.getUserId();
            Test.startTest();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 27, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P5d'));
            Test.stopTest();
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 27, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void a_c5e_register_prev() {
        String obs = 'C5e';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 28, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P5e'));
            obs += ' registered';
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void b_c5f_use_prev() {
        String obs = 'C5f';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 28, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

}
