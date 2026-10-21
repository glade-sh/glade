package apextest

import "testing"

func TestLoopLocalInitializerDoesNotUsePriorIterationShadow(t *testing.T) {
	project := newSalesforceSurfaceProject(t, `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"66.0"}`)
	project.writeClass("TokenProbe", `public class TokenProbe {
  public static final String DEFAULT_VALUE = 'default';
  public String value;
  public TokenProbe(String value) { this.value = value; }
}`)
	project.writeClass("TokenProbeTest", `@IsTest private class TokenProbeTest {
  @IsTest static void repeatedInitializer() {
    List<String> observed = new List<String>();
    for (String name : new List<String>{'first','second','third'}) {
      TokenProbe tokenProbe = new TokenProbe(TokenProbe.DEFAULT_VALUE);
      observed.add(tokenProbe.value);
    }
    System.assertEquals(new List<String>{'default','default','default'},observed);
  }
}`)
	run := project.run()
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		for _, suite := range run.Suites {
			for _, test := range suite.Cases {
				if test.Problem != nil {
					t.Logf("%s: %#v", test.MethodName, *test.Problem)
				}
			}
		}
		t.Fatalf("summary = %#v suites = %#v", got, run.Suites)
	}
}
