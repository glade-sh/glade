package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// Owned native twins cover homogeneous API62/67/68 helpers and callers.
func TestH09ReceiverBoundaryPrecedence(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			for name, source := range h09ReceiverBoundarySources {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), source)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"OwnedReceiverBoundaryTest"}})
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					t.Run(result.MethodName, func(t *testing.T) {
						if result.Status != testreport.StatusPass {
							data, _ := json.Marshal(result)
							t.Fatalf("native receiver boundary: %s", data)
						}
					})
				}
			}
			if got := run.Summary(); got.Total != 6 || got.Passed != 6 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
				data, _ := json.Marshal(run)
				t.Fatalf("receiver boundary contracts: %s", data)
			}
		})
	}
}

var h09ReceiverBoundarySources = map[string]string{
	"AtlasBase": `public virtual class AtlasBase {
    public String forward(Type token) {
        return TokenFactory.create(token);
    }

    public String local(Type token) {
        GlyphReceiver TokenFactory = new GlyphReceiver();
        return TokenFactory.create(token);
    }

    public class PublicChild extends AtlasBase {
        public String tokenFactory = 'public state';

        public String ownField() {
            return tokenFactory.toUpperCase();
        }

        public String explicitField() {
            return this.tokenFactory.toUpperCase();
        }
    }

    public class ProtectedChild extends AtlasBase {
        protected String tokenFactory = 'protected state';

        public String ownField() {
            return tokenFactory.toUpperCase();
        }

        public String explicitField() {
            return this.tokenFactory.toUpperCase();
        }
    }

    public class Child extends AtlasBase {
        private String tokenFactory = 'nested state';

        public String ownField() {
            return tokenFactory.toUpperCase();
        }

        public String explicitField() {
            return this.tokenFactory.toUpperCase();
        }
    }
}
`,
	"AtlasFieldBase": `public virtual class AtlasFieldBase {
    private String tokenFactory = 'base state';

    public String readImplicit() {
        return tokenFactory.toUpperCase();
    }

    public String readExplicit() {
        return this.tokenFactory.toUpperCase();
    }

    public class Child extends AtlasFieldBase {
        private String tokenFactory = 'derived state';

        public String ownField() {
            return tokenFactory.toUpperCase();
        }

        public String explicitField() {
            return this.tokenFactory.toUpperCase();
        }
    }
}
`,
	"GlyphChild": `public class GlyphChild extends AtlasBase {
    private String tokenFactory = 'child state';

    public String ownField() {
        return tokenFactory.toUpperCase();
    }

    public String explicitField() {
        return this.tokenFactory.toUpperCase();
    }
}
`,
	"GlyphReceiver": `public class GlyphReceiver {
    public String create(Type token) {
        return 'local:' + token.getName();
    }
}
`,
	"TokenFactory": `public class TokenFactory {
    public static String create(Type token) {
        return token.getName();
    }
}
`,
	"OwnedReceiverBoundaryTest": `@IsTest
private class OwnedReceiverBoundaryTest {
    @IsTest static void publicDerivedForward() {
        AtlasBase.PublicChild value = new AtlasBase.PublicChild();
        for (Type token : new List<Type>{String.class, List<String>.class, Map<String, List<Integer>>.class}) {
            System.assertEquals(TokenFactory.create(token), value.forward(token));
        }
    }

    @IsTest static void protectedDerivedForward() {
        AtlasBase.ProtectedChild value = new AtlasBase.ProtectedChild();
        for (Type token : new List<Type>{String.class, List<String>.class, Map<String, List<Integer>>.class}) {
            System.assertEquals(TokenFactory.create(token), value.forward(token));
        }
    }

    @IsTest static void lexicalBaseFieldInherited() {
        AtlasFieldBase baseValue = new AtlasFieldBase();
        AtlasFieldBase.Child childValue = new AtlasFieldBase.Child();
        System.assertEquals('BASE STATE', baseValue.readImplicit());
        System.assertEquals('BASE STATE', baseValue.readExplicit());
        System.assertEquals('BASE STATE', childValue.readImplicit());
        System.assertEquals('BASE STATE', childValue.readExplicit());
    }

    @IsTest static void localReceiverControl() {
        AtlasBase value = new AtlasBase();
        for (Type token : new List<Type>{String.class, List<String>.class, Map<String, List<Integer>>.class}) {
            System.assertEquals('local:' + token.getName(), value.local(token));
        }
    }

    @IsTest static void derivedOwnAndExplicitControl() {
        AtlasBase.PublicChild publicValue = new AtlasBase.PublicChild();
        AtlasBase.ProtectedChild protectedValue = new AtlasBase.ProtectedChild();
        AtlasFieldBase.Child fieldChild = new AtlasFieldBase.Child();
        System.assertEquals('PUBLIC STATE', publicValue.ownField());
        System.assertEquals('PUBLIC STATE', publicValue.explicitField());
        System.assertEquals('PROTECTED STATE', protectedValue.ownField());
        System.assertEquals('PROTECTED STATE', protectedValue.explicitField());
        System.assertEquals('DERIVED STATE', fieldChild.ownField());
        System.assertEquals('DERIVED STATE', fieldChild.explicitField());
    }

    @IsTest static void privateFactoryControl() {
        GlyphChild childValue = new GlyphChild();
        AtlasBase.Child nestedValue = new AtlasBase.Child();
        for (Type token : new List<Type>{String.class, List<String>.class, Map<String, List<Integer>>.class}) {
            System.assertEquals(TokenFactory.create(token), childValue.forward(token));
            System.assertEquals(TokenFactory.create(token), nestedValue.forward(token));
        }
        System.assertEquals('CHILD STATE', childValue.ownField());
        System.assertEquals('CHILD STATE', childValue.explicitField());
        System.assertEquals('NESTED STATE', nestedValue.ownField());
        System.assertEquals('NESTED STATE', nestedValue.explicitField());
    }
}
`,
}
