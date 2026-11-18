@IsTest
private class CA_SiloedA67 {
    @IsTest
    static void c1a_same_instance() {
        String obs = 'C1a';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedElementPage p = CA_UtilA67.page('P1a');
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, p);
            ConnectApi.FeedElementPage g1 = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            ConnectApi.FeedElementPage g2 = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            obs += ' identity=' + String.valueOf(g1 === p) + ' sameAgain=' + String.valueOf(g2 === g1) + ' url=' + g1.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c1b_mutation_visible() {
        String obs = 'C1b';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedElementPage p = CA_UtilA67.page('P1b');
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, p);
            p.currentPageUrl = 'MUTATED';
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            obs += ' urlAfterMutate=' + g.currentPageUrl + ' identity=' + String.valueOf(g === p);
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c1c_two_keys() {
        String obs = 'C1c';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedElementPage a = CA_UtilA67.page('K1');
            ConnectApi.FeedElementPage b = CA_UtilA67.page('K2');
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, a);
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'me', b);
            ConnectApi.FeedElementPage g1 = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            ConnectApi.FeedElementPage g2 = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'me');
            obs += ' urlA=' + g1.currentPageUrl + ' urlB=' + g2.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2a_subject_diff() {
        String obs = 'C2a';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('P2a'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'me');
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2b_feedtype_diff() {
        String obs = 'C2b';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('P2b'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.Company, uid);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2c_extra_optional() {
        String obs = 'C2c';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('P2c'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2d_null_vs_me() {
        String obs = 'C2d';
        try {
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, (String)null, CA_UtilA67.page('P2d'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'me');
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2e_me_vs_null() {
        String obs = 'C2e';
        try {
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'me', CA_UtilA67.page('P2e'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, (String)null);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2f_me_vs_userid() {
        String obs = 'C2f';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'me', CA_UtilA67.page('P2f'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2g_userid_exact() {
        String obs = 'C2g';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedElementPage p = CA_UtilA67.page('P2g');
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, p);
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            obs += ' OK url=' + g.currentPageUrl + ' identity=' + String.valueOf(g === p);
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2h_pagesize_diff() {
        String obs = 'C2h';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P2h'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 50, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2i_pagesize_exact() {
        String obs = 'C2i';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P2i'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2j_pageparam_diff() {
        String obs = 'C2j';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, 'PAGE2', 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P2j'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2k_sort_diff() {
        String obs = 'C2k';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P2k'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.CreatedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2l_invalid_subject_validated() {
        String obs = 'C2l';
        try {
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'notanid');
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2m_settest_stores_any_value() {
        String obs = 'C2m';
        try {
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'notanid', CA_UtilA67.page('P2m'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'notanid');
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2n_me_exact() {
        String obs = 'C2n';
        try {
            ConnectApi.FeedElementPage p = CA_UtilA67.page('P2n');
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'me', p);
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, 'me');
            obs += ' OK url=' + g.currentPageUrl + ' identity=' + String.valueOf(g === p);
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c2o_fake_005_id() {
        String obs = 'C2o';
        try {
            String fake = '005000000000001AAA';
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, fake, CA_UtilA67.page('P2o'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, fake);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c3a_no_reg() {
        String obs = 'C3a';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c3b_no_reg_other_overload() {
        String obs = 'C3b';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c4a_twice_same_args() {
        String obs = 'C4a';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('A'));
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('B'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            obs += ' url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c4b_a_get_b_get() {
        String obs = 'C4b';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('A'));
            ConnectApi.FeedElementPage g1 = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('B'));
            ConnectApi.FeedElementPage g2 = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            obs += ' url1=' + g1.currentPageUrl + ' url2=' + g2.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c4c_cross_overload() {
        String obs = 'C4c';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('A'));
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('B'));
            ConnectApi.FeedElementPage g3 = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            ConnectApi.FeedElementPage g6 = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' url3=' + g3.currentPageUrl + ' url6=' + g6.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c6a_set4_get3() {
        String obs = 'C6a';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('P6a'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c6b_set4_get6() {
        String obs = 'C6b';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('P6b'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c6c_set6_get3() {
        String obs = 'C6c';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P6c'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c6d_set6_get6() {
        String obs = 'C6d';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P6d'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c6e_set_density_get_density() {
        String obs = 'C6e';
        try {
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, (Integer)null, (ConnectApi.FeedDensity)null, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P6e'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, (Integer)null, (ConnectApi.FeedDensity)null, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c6f_set_density_get_subject() {
        String obs = 'C6f';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, (Integer)null, (ConnectApi.FeedDensity)null, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P6f'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7a_favorites_create() {
        String obs = 'C7a';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedFavorite f = ConnectApi.ChatterFavorites.addFavorite((String)null, uid, 'gladecap');
            obs += ' created=' + String.valueOf(f != null) + ' idPrefix=' + (f == null || f.id == null ? 'null' : f.id.substring(0, 3));
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7b_favorites_hit() {
        String obs = 'C7b';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedFavorite f = ConnectApi.ChatterFavorites.addFavorite((String)null, uid, 'gladecap1');
            ConnectApi.FeedElementPage p = CA_UtilA67.page('P7b');
            ConnectApi.ChatterFavorites.setTestGetFeedElements((String)null, uid, f.id, p);
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFavorites.getFeedElements((String)null, uid, f.id);
            obs += ' identity=' + String.valueOf(g === p) + ' url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7c_favorites_diff() {
        String obs = 'C7c';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedFavorite f1 = ConnectApi.ChatterFavorites.addFavorite((String)null, uid, 'gladecap21');
            ConnectApi.FeedFavorite f2 = ConnectApi.ChatterFavorites.addFavorite((String)null, uid, 'gladecap22');
            ConnectApi.ChatterFavorites.setTestGetFeedElements((String)null, uid, f1.id, CA_UtilA67.page('P7c'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFavorites.getFeedElements((String)null, uid, f2.id);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7d_favorites_set3_get6() {
        String obs = 'C7d';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedFavorite f = ConnectApi.ChatterFavorites.addFavorite((String)null, uid, 'gladecap3');
            ConnectApi.ChatterFavorites.setTestGetFeedElements((String)null, uid, f.id, CA_UtilA67.page('P7d'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFavorites.getFeedElements((String)null, uid, f.id, (String)null, 25, ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7e_favorites_no_reg() {
        String obs = 'C7e';
        try {
            String uid = UserInfo.getUserId();
            ConnectApi.FeedFavorite f = ConnectApi.ChatterFavorites.addFavorite((String)null, uid, 'gladecap4');
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFavorites.getFeedElements((String)null, uid, f.id);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7f_search_hit() {
        String obs = 'C7f';
        try {
            ConnectApi.FeedElementPage p = CA_UtilA67.page('P7f');
            ConnectApi.ChatterFeeds.setTestSearchFeedElements((String)null, 'gladecap', p);
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.searchFeedElements((String)null, 'gladecap');
            obs += ' identity=' + String.valueOf(g === p) + ' url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7g_search_sort_miss() {
        String obs = 'C7g';
        try {
            ConnectApi.ChatterFeeds.setTestSearchFeedElements((String)null, 'gladecap2', CA_UtilA67.page('P7g'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.searchFeedElements((String)null, 'gladecap2', ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7h_search_sort_hit() {
        String obs = 'C7h';
        try {
            ConnectApi.ChatterFeeds.setTestSearchFeedElements((String)null, 'gladecap3', ConnectApi.FeedSortOrder.LastModifiedDateDesc, CA_UtilA67.page('P7h'));
            ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.searchFeedElements((String)null, 'gladecap3', ConnectApi.FeedSortOrder.LastModifiedDateDesc);
            obs += ' OK url=' + g.currentPageUrl;
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7i_moderation_get() {
        String obs = 'C7i';
        try {
            ConnectApi.ModerationCapability mc = ConnectApi.CommunityModeration.getFlagsOnFeedElement((String)null, '0D5000000000001AAA');
            obs += ' OK value=' + String.valueOf(mc == null ? 'null' : 'object');
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7j_knowledge_hit() {
        String obs = 'C7j';
        try {
            ConnectApi.KnowledgeArticleVersionCollection reg = CA_UtilA67.articles('K1');
            ConnectApi.Knowledge.setTestGetTrendingArticles((String)null, 5, reg);
            ConnectApi.KnowledgeArticleVersionCollection g = ConnectApi.Knowledge.getTrendingArticles((String)null, 5);
            obs += ' identity=' + String.valueOf(g === reg) + ' items=' + String.valueOf(g.items == null ? -1 : g.items.size());
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7k_knowledge_max_diff() {
        String obs = 'C7k';
        try {
            ConnectApi.Knowledge.setTestGetTrendingArticles((String)null, 5, CA_UtilA67.articles('K2'));
            ConnectApi.KnowledgeArticleVersionCollection g = ConnectApi.Knowledge.getTrendingArticles((String)null, 10);
            obs += ' OK items=' + String.valueOf(g.items == null ? -1 : g.items.size());
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7l_knowledge_topic_hit() {
        String obs = 'C7l';
        try {
            Topic t = new Topic(Name = 'gladecap', Description = 'glade capture');
            insert t;
            ConnectApi.KnowledgeArticleVersionCollection reg = CA_UtilA67.articles('K3');
            ConnectApi.Knowledge.setTestGetTrendingArticlesForTopic((String)null, t.Id, 5, reg);
            ConnectApi.KnowledgeArticleVersionCollection g = ConnectApi.Knowledge.getTrendingArticlesForTopic((String)null, t.Id, 5);
            obs += ' identity=' + String.valueOf(g === reg) + ' items=' + String.valueOf(g.items == null ? -1 : g.items.size()) + ' topicPrefix=' + String.valueOf(t.Id).substring(0, 3);
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c7m_knowledge_topic_fake() {
        String obs = 'C7m';
        try {
            ConnectApi.KnowledgeArticleVersionCollection g = ConnectApi.Knowledge.getTrendingArticlesForTopic((String)null, '0TO000000000001AAA', 5);
            obs += ' OK items=' + String.valueOf(g.items == null ? -1 : g.items.size());
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c8a_limits_registered() {
        String obs = 'C8a';
        try {
            String uid = UserInfo.getUserId();
            Integer q0 = Limits.getQueries();
            Integer c0 = Limits.getCallouts();
            Integer d0 = Limits.getDmlStatements();
            Integer s0 = Limits.getSoslQueries();
            Integer t0 = Limits.getCpuTime();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('P8a'));
            for (Integer i = 0; i < 5; i++) {
                ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            }
            Integer q1 = Limits.getQueries();
            Integer c1 = Limits.getCallouts();
            Integer d1 = Limits.getDmlStatements();
            Integer s1 = Limits.getSoslQueries();
            Integer t1 = Limits.getCpuTime();
            obs += ' dQueries=' + (q1 - q0) + ' dCallouts=' + (c1 - c0) + ' dDml=' + (d1 - d0) + ' dSosl=' + (s1 - s0) + ' dCpu=' + (t1 - t0);
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c8b_limits_unregistered() {
        String obs = 'C8b';
        try {
            String uid = UserInfo.getUserId();
            Integer q0 = Limits.getQueries();
            Integer c0 = Limits.getCallouts();
            Integer t0 = Limits.getCpuTime();
            try {
                ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            } catch (Exception innerEx) {
                obs += ' inner=' + innerEx.getTypeName();
            }
            Integer q1 = Limits.getQueries();
            Integer c1 = Limits.getCallouts();
            Integer t1 = Limits.getCpuTime();
            obs += ' dQueries=' + (q1 - q0) + ' dCallouts=' + (c1 - c0) + ' dCpu=' + (t1 - t0);
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

    @IsTest
    static void c8c_limits_ten() {
        String obs = 'C8c';
        try {
            String uid = UserInfo.getUserId();
            Integer q0 = Limits.getQueries();
            Integer c0 = Limits.getCallouts();
            Integer d0 = Limits.getDmlStatements();
            ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid, CA_UtilA67.page('P8c'));
            for (Integer i = 0; i < 10; i++) {
                ConnectApi.FeedElementPage g = ConnectApi.ChatterFeeds.getFeedElementsFromFeed((String)null, ConnectApi.FeedType.News, uid);
            }
            Integer q1 = Limits.getQueries();
            Integer c1 = Limits.getCallouts();
            Integer d1 = Limits.getDmlStatements();
            obs += ' dQueries=' + (q1 - q0) + ' dCallouts=' + (c1 - c0) + ' dDml=' + (d1 - d0);
        } catch (Exception e) {
            obs += ' THREW ' + CA_UtilA67.ex(e);
        }
        System.assert(false, obs);
    }

}
