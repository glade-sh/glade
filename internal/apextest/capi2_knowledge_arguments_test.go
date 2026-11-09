package apextest

import "testing"

// capi2_knowledge_arguments/org.tsv records the argument checks that run
// before the data-siloed boundary for the Knowledge service. The same rows hold
// at APIs 62.0, 67.0 and 68.0.
func TestCapi2KnowledgeArguments(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			p.writeClass("KnowledgeArgsTest", `@IsTest private class KnowledgeArgsTest {
  static String top(String c, String t) { try { ConnectApi.Knowledge.getTopViewedArticlesForTopic(c, t, 1); return 'OK'; } catch (Exception e) { return e.getMessage().replace(UserInfo.getUserId(), '{userId}'); } }
  static String trending(String c) { try { ConnectApi.Knowledge.getTrendingArticles(c, 1); return 'OK'; } catch (Exception e) { return e.getMessage().replace(UserInfo.getUserId(), '{userId}'); } }
  static String trendTopic(String c, String t) { try { ConnectApi.Knowledge.getTrendingArticlesForTopic(c, t, 1); return 'OK'; } catch (Exception e) { return e.getMessage().replace(UserInfo.getUserId(), '{userId}'); } }
  @IsTest static void probe() {
    String silo = 'ConnectApi methods are not supported in data siloed tests. Please use @IsTest(SeeAllData=true).';
    System.assertEquals('Parameter required: topicId', top(null, null));
    System.assertEquals('Parameter required: topicId', trendTopic(null, null));
    System.assertEquals('Illegal value for parameter: \'communityId\': {userId}', top(UserInfo.getUserId(), null));
    System.assertEquals('Illegal value for parameter: \'communityId\': {userId}', trending(UserInfo.getUserId()));
    System.assertEquals('Illegal value for parameter: \'communityId\': {userId}', trendTopic(UserInfo.getUserId(), '0TO000000000001'));
    System.assertEquals('Illegal value for parameter: \'communityId\': me', trending('me'));
    System.assertEquals('Illegal value for parameter: \'communityId\': abc', top('abc', null));
    System.assertEquals('Illegal value for parameter: \'communityId\': 0TO000000000001', trendTopic('0TO000000000001', null));
    System.assertEquals('Illegal value for parameter: \'topicId\': {userId}', top(null, UserInfo.getUserId()));
    System.assertEquals('Illegal value for parameter: \'topicId\': {userId}', trendTopic(null, UserInfo.getUserId()));
    System.assertEquals('Illegal value for parameter: \'topicId\': me', top(null, 'me'));
    System.assertEquals('Illegal value for parameter: \'topicId\': abc', trendTopic(null, 'abc'));
    System.assertEquals(silo, top(null, '0TO000000000001'));
    // Valid trending arguments end the test with an uncatchable "No matching test result" failure.
  }
}`)
			r := p.run()
			if s := r.Summary(); s.Total != 1 || s.Passed != 1 {
				t.Fatalf("summary=%#v problem=%q", s, firstRunProblem(r))
			}
		})
	}
}
