package vm

import "testing"

// Salesforce resolves overloads by the static type of the argument, so a
// platform method with a declared return type must still carry that type when
// it returns null (see TestOverloadStaticIdTypeForNullReturns for the native
// rows). Each case feeds a null platform return into f(Id)/f(String)/
// f(Integer)/f(Date)/f(PageReference) and expects the overload that matches
// the declared return type.
func TestOverloadStaticTypeForNullPlatformReturns(t *testing.T) {
	for _, tc := range []struct{ name, call, want string }{
		{"Packaging.getCurrentPackageId", "r = Q.f(Packaging.getCurrentPackageId());", "Id"},
		{"UserInfo.getProfileId", "r = Q.f(UserInfo.getProfileId());", "Id"},
		{"UserInfo.getUserRoleId", "r = Q.f(UserInfo.getUserRoleId());", "Id"},
		{"UserInfo.getCurrentUvid", "r = Q.f(UserInfo.getCurrentUvid());", "String"},
		{"Site.getSiteId", "r = Q.f(Site.getSiteId());", "Id"},
		{"Site.getAdminId", "r = Q.f(Site.getAdminId());", "Id"},
		{"Site.getName", "r = Q.f(Site.getName());", "String"},
		{"Site.getAdminEmail", "r = Q.f(Site.getAdminEmail());", "String"},
		{"Site.getOriginalUrl", "r = Q.f(Site.getOriginalUrl());", "String"},
		{"Network.getNetworkId", "r = Q.f(Network.getNetworkId());", "Id"},
		{"Network.getLoginUrl", "r = Q.f(Network.getLoginUrl('x'));", "String"},
		{"Network.getSelfRegUrl", "r = Q.f(Network.getSelfRegUrl('x'));", "String"},
		{"Site.login", "r = Q.f(Site.login('u', 'p', null));", "PageReference"},
		{"Matcher.group non-participating", "Matcher m = Pattern.compile('(a)|(b)').matcher('a'); m.matches(); r = Q.f(m.group(2));", "String"},
		{"Datetime.valueOf((Object)null)", "Object o = null; r = Q.f(Datetime.valueOf(o));", "Datetime"},
		{"Integer.valueOf((Object)null)", "Object o = null; r = Q.f(Integer.valueOf(o));", "Integer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			if err := machine.RegisterClass(Class{Name: "Q"}); err != nil {
				t.Fatal(err)
			}
			for _, spec := range []string{"Id", "String", "Integer", "Datetime", "PageReference"} {
				body, err := CompileAnonymous("return '" + spec + "';")
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{Name: "Q.f", ClassName: "Q", IsStatic: true, ReturnType: "String", Params: []Param{{Name: "value", Type: spec}}, Program: body}); err != nil {
					t.Fatal(err)
				}
			}
			program, err := CompileAnonymous("String r; " + tc.call + " System.assertEquals('" + tc.want + "', r);")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
