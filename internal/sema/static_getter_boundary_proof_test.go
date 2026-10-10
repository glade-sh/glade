package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// These complete owned projects retain the declaring and caller component APIs.
// Runtime assertions stay in the versioned fixture; this gate checks compilation.
func TestStaticGetterExternalAssignmentComponentBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, fixture string
		reject        bool
	}{
		{name: "static-getter-decl41-caller41-full-runtime", reject: false, fixture: `{
  "name": "static-getter-decl41-caller41-full-runtime",
  "apiVersion": "41.0",
  "project": {
    "sourceApiVersion": "41.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeStaticFull41C41.cls",
      "content": "public class GladeStaticFull41C41 {\n public String marker;\n public static GladeStaticFull41C41 Instance { get { if (Instance == null) { Instance = new GladeStaticFull41C41(); } return Instance; } }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticFull41C41.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>41.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticFull41C41Proof.cls",
      "content": "@IsTest private class GladeStaticFull41C41Proof {\n @IsTest static void externalGetterAssignmentObserver() {\n  GladeStaticFull41C41 replacement = new GladeStaticFull41C41(); replacement.marker = 'owned-replacement';\n  GladeStaticFull41C41.Instance = replacement;\n  System.assertEquals('owned-replacement', GladeStaticFull41C41.Instance.marker); System.assert(GladeStaticFull41C41.Instance === replacement);\n }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticFull41C41Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>41.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "test"
  },
  "expected": {
    "result": {
      "ok": true,
      "errors": 0,
      "failed": 0,
      "passed": 1,
      "total": 1
    }
  }
}
`},
		{name: "static-getter-decl24-caller41-full-runtime", reject: false, fixture: `{
  "name": "static-getter-decl24-caller41-full-runtime",
  "apiVersion": "41.0",
  "project": {
    "sourceApiVersion": "41.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeStaticFull24C41.cls",
      "content": "public class GladeStaticFull24C41 {\n public String marker;\n public static GladeStaticFull24C41 Instance { get { if (Instance == null) { Instance = new GladeStaticFull24C41(); } return Instance; } }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticFull24C41.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>24.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticFull24C41Proof.cls",
      "content": "@IsTest private class GladeStaticFull24C41Proof {\n @IsTest static void externalGetterAssignmentObserver() {\n  GladeStaticFull24C41 replacement = new GladeStaticFull24C41(); replacement.marker = 'owned-replacement';\n  GladeStaticFull24C41.Instance = replacement;\n  System.assertEquals('owned-replacement', GladeStaticFull24C41.Instance.marker); System.assert(GladeStaticFull24C41.Instance === replacement);\n }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticFull24C41Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>41.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "test"
  },
  "expected": {
    "result": {
      "ok": true,
      "errors": 0,
      "failed": 0,
      "passed": 1,
      "total": 1
    }
  }
}
`},
		{name: "static-getter-decl41-caller24-full-runtime", reject: false, fixture: `{
  "name": "static-getter-decl41-caller24-full-runtime",
  "apiVersion": "24.0",
  "project": {
    "sourceApiVersion": "24.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeStaticFull41C24.cls",
      "content": "public class GladeStaticFull41C24 {\n public String marker;\n public static GladeStaticFull41C24 Instance { get { if (Instance == null) { Instance = new GladeStaticFull41C24(); } return Instance; } }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticFull41C24.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>41.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticFull41C24Proof.cls",
      "content": "@IsTest private class GladeStaticFull41C24Proof {\n @IsTest static void externalGetterAssignmentObserver() {\n  GladeStaticFull41C24 replacement = new GladeStaticFull41C24(); replacement.marker = 'owned-replacement';\n  GladeStaticFull41C24.Instance = replacement;\n  System.assertEquals('owned-replacement', GladeStaticFull41C24.Instance.marker); System.assert(GladeStaticFull41C24.Instance === replacement);\n }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticFull41C24Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>24.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "test"
  },
  "expected": {
    "result": {
      "ok": true,
      "errors": 0,
      "failed": 0,
      "passed": 1,
      "total": 1
    }
  }
}
`},
		{name: "static-getter-decl24-caller42-external-only", reject: true, fixture: `{
  "name": "static-getter-decl24-caller42-external-only",
  "apiVersion": "42.0",
  "project": {
    "sourceApiVersion": "42.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary24C42.cls",
      "content": "public class GladeStaticBoundary24C42 {\n private static GladeStaticBoundary24C42 backing = new GladeStaticBoundary24C42();\n public static GladeStaticBoundary24C42 Instance { get { return backing; } }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary24C42.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>24.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary24C42Proof.cls",
      "content": "@IsTest private class GladeStaticBoundary24C42Proof {\n @IsTest static void externalAssignmentCompileObserver() {\n  GladeStaticBoundary24C42 replacement = new GladeStaticBoundary24C42();\n  GladeStaticBoundary24C42.Instance = replacement;\n }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary24C42Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>42.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "check"
  },
  "expected": {
    "result": {
      "ok": true
    }
  }
}
`},
		{name: "static-getter-decl41-caller42-external-only", reject: true, fixture: `{
  "name": "static-getter-decl41-caller42-external-only",
  "apiVersion": "42.0",
  "project": {
    "sourceApiVersion": "42.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary41C42.cls",
      "content": "public class GladeStaticBoundary41C42 {\n private static GladeStaticBoundary41C42 backing = new GladeStaticBoundary41C42();\n public static GladeStaticBoundary41C42 Instance { get { return backing; } }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary41C42.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>41.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary41C42Proof.cls",
      "content": "@IsTest private class GladeStaticBoundary41C42Proof {\n @IsTest static void externalAssignmentCompileObserver() {\n  GladeStaticBoundary41C42 replacement = new GladeStaticBoundary41C42();\n  GladeStaticBoundary41C42.Instance = replacement;\n }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary41C42Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>42.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "check"
  },
  "expected": {
    "result": {
      "ok": true
    }
  }
}
`},
		{name: "static-getter-decl42-caller24-external-only", reject: true, fixture: `{
  "name": "static-getter-decl42-caller24-external-only",
  "apiVersion": "24.0",
  "project": {
    "sourceApiVersion": "24.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C24.cls",
      "content": "public class GladeStaticBoundary42C24 {\n private static GladeStaticBoundary42C24 backing = new GladeStaticBoundary42C24();\n public static GladeStaticBoundary42C24 Instance { get { return backing; } }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C24.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>42.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C24Proof.cls",
      "content": "@IsTest private class GladeStaticBoundary42C24Proof {\n @IsTest static void externalAssignmentCompileObserver() {\n  GladeStaticBoundary42C24 replacement = new GladeStaticBoundary42C24();\n  GladeStaticBoundary42C24.Instance = replacement;\n }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C24Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>24.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "check"
  },
  "expected": {
    "result": {
      "ok": true
    }
  }
}
`},
		{name: "static-getter-decl42-caller42-external-only", reject: true, fixture: `{
  "name": "static-getter-decl42-caller42-external-only",
  "apiVersion": "42.0",
  "project": {
    "sourceApiVersion": "42.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C42.cls",
      "content": "public class GladeStaticBoundary42C42 {\n private static GladeStaticBoundary42C42 backing = new GladeStaticBoundary42C42();\n public static GladeStaticBoundary42C42 Instance { get { return backing; } }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C42.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>42.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C42Proof.cls",
      "content": "@IsTest private class GladeStaticBoundary42C42Proof {\n @IsTest static void externalAssignmentCompileObserver() {\n  GladeStaticBoundary42C42 replacement = new GladeStaticBoundary42C42();\n  GladeStaticBoundary42C42.Instance = replacement;\n }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C42Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>42.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "check"
  },
  "expected": {
    "result": {
      "ok": true
    }
  }
}
`},
		{name: "static-getter-decl42-caller41-external-only", reject: true, fixture: `{
  "name": "static-getter-decl42-caller41-external-only",
  "apiVersion": "41.0",
  "project": {
    "sourceApiVersion": "41.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C41.cls",
      "content": "public class GladeStaticBoundary42C41 {\n private static GladeStaticBoundary42C41 backing = new GladeStaticBoundary42C41();\n public static GladeStaticBoundary42C41 Instance { get { return backing; } }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C41.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>42.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C41Proof.cls",
      "content": "@IsTest private class GladeStaticBoundary42C41Proof {\n @IsTest static void externalAssignmentCompileObserver() {\n  GladeStaticBoundary42C41 replacement = new GladeStaticBoundary42C41();\n  GladeStaticBoundary42C41.Instance = replacement;\n }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeStaticBoundary42C41Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>41.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "check"
  },
  "expected": {
    "result": {
      "ok": true
    }
  }
}
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var fixture struct {
				Project json.RawMessage                  `json:"project"`
				Source  []struct{ Path, Content string } `json:"source"`
			}
			if err := json.Unmarshal([]byte(tc.fixture), &fixture); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), string(fixture.Project))
			for _, file := range fixture.Source {
				path := filepath.Join(root, filepath.FromSlash(file.Path))
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				writeSemaFile(t, path, file.Content)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(typesys.Build(p, schema.Schema{}))
			if tc.reject {
				if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA019" || !strings.Contains(result.Diagnostics[0].Message, "property has no setter") {
					t.Fatalf("expected exactly the missing-setter rejection: %#v", result.Diagnostics)
				}
			} else if result.HasErrors() {
				t.Fatalf("legacy declaring/caller project rejected: %#v", result.Diagnostics)
			}
		})
	}
}

// These local guard regressions preserve the existing receiver restrictions;
// they do not extend the admitted class-qualified version contract.
func TestStaticGetterExternalAssignmentReceiverGuards(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"instance receiver", "Holder instance = new Holder(); instance.value = 'changed';"},
		{"chained instance receiver", "Holder.instance.value = 'changed';"},
		{"local shadows type name", "Holder Holder = new Holder(); Holder.value = 'changed';"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Holder.cls": "public class Holder { public static Holder instance = new Holder(); public static String value { get { return 'ready'; } } }",
				"Caller.cls": "public class Caller { public static void run() { " + tc.body + " } }",
			}, "41.0")
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Code == "GLADESEMA019" && strings.Contains(diagnostic.Message, "property has no setter") {
					return
				}
			}
			t.Fatalf("non-type receiver no longer preserves missing-setter rejection: %#v", result.Diagnostics)
		})
	}
}
