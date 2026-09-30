package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// These complete source/version populations were deployed and all named tests
// passed on Salesforce. This check preserves the original fixture bytes.
func TestSalesforceAdmittedReadonlyContracts(t *testing.T) {
	for _, tc := range []struct{ name, fixture string }{
		{name: "readonly-reads-api24", fixture: `{
  "name": "readonly-reads-api24",
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
      "path": "force-app/main/default/classes/GladeReadonlyReads24.cls",
      "content": "@IsTest private class GladeReadonlyReads24 {\n@IsTest static void accountMasterRecordEqualityIsRead() {\n Account account = new Account(); Integer count = 0;\n if (account.MasterRecordId == null) { count++; }\n System.assertEquals(1, count);\n account = (Account)JSON.deserialize('{\"MasterRecordId\":\"001000000000001\"}', Account.class);\n if (account.MasterRecordId == null) { count++; }\n System.assertEquals(1, count);\n System.assertEquals((Id)'001000000000001', account.MasterRecordId);\n}\n@IsTest static void emailTemplateTypeEqualityIsRead() {\n EmailTemplate template = (EmailTemplate)JSON.deserialize('{\"TemplateType\":\"html\"}', EmailTemplate.class);\n Integer count = 0;\n if (template.TemplateType == 'html') { count++; }\n System.assertEquals(1, count);\n template = (EmailTemplate)JSON.deserialize('{\"TemplateType\":\"text\"}', EmailTemplate.class);\n if (template.TemplateType == 'html') { count++; }\n System.assertEquals(1, count); System.assertEquals('text', template.TemplateType);\n}\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeReadonlyReads24.cls-meta.xml",
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
      "passed": 2,
      "total": 2
    }
  }
}
`},
		{name: "readonly-reads-api31", fixture: `{
  "name": "readonly-reads-api31",
  "apiVersion": "31.0",
  "project": {
    "sourceApiVersion": "31.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeReadonlyReads31.cls",
      "content": "@IsTest private class GladeReadonlyReads31 {\n@IsTest static void cronStateEqualityWithOrIsRead() {\n CronTrigger ct = (CronTrigger)JSON.deserialize('{\"State\":\"DELETED\",\"NextFireTime\":\"2025-01-01T00:00:00.000Z\"}', CronTrigger.class);\n Integer count = 0;\n if (ct.State == 'DELETED' || ct.NextFireTime == null) { count++; }\n System.assertEquals(1, count);\n ct = (CronTrigger)JSON.deserialize('{\"State\":\"WAITING\",\"NextFireTime\":\"2025-01-01T00:00:00.000Z\"}', CronTrigger.class);\n if (ct.State == 'DELETED' || ct.NextFireTime == null) { count++; }\n System.assertEquals(1, count);\n}\n@IsTest static void cronElseIfEqualityIsRead() {\n CronTrigger c = (CronTrigger)JSON.deserialize('{\"State\":\"DELETED\"}', CronTrigger.class); Integer branch = 0;\n if (c.NextFireTime != null) { branch = 1; } else if (c.State == 'DELETED') { branch = 2; }\n System.assertEquals(2, branch);\n c = (CronTrigger)JSON.deserialize('{\"State\":\"WAITING\"}', CronTrigger.class); branch = 0;\n if (c.NextFireTime != null) { branch = 1; } else if (c.State == 'DELETED') { branch = 2; }\n System.assertEquals(0, branch);\n}\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeReadonlyReads31.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>31.0</apiVersion><status>Active</status></ApexClass>\n"
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
      "passed": 2,
      "total": 2
    }
  }
}
`},
		{name: "readonly-reads-api37", fixture: `{
  "name": "readonly-reads-api37",
  "apiVersion": "37.0",
  "project": {
    "sourceApiVersion": "37.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeReadonlyReads37.cls",
      "content": "@IsTest private class GladeReadonlyReads37 {\n@IsTest static void asyncErrorEqualityIsRead() {\n AsyncApexJob job = (AsyncApexJob)JSON.deserialize('{\"NumberOfErrors\":0}', AsyncApexJob.class); Integer count = 0;\n if (job.NumberOfErrors == 0) { count++; }\n System.assertEquals(1, count);\n job = (AsyncApexJob)JSON.deserialize('{\"NumberOfErrors\":2}', AsyncApexJob.class);\n if (job.NumberOfErrors == 0) { count++; }\n System.assertEquals(1, count); System.assertEquals(2, job.NumberOfErrors);\n}\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeReadonlyReads37.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>37.0</apiVersion><status>Active</status></ApexClass>\n"
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
		{name: "readonly-reads-api48", fixture: `{
  "name": "readonly-reads-api48",
  "apiVersion": "48.0",
  "project": {
    "sourceApiVersion": "48.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeReadonlyReads48.cls",
      "content": "@IsTest private class GladeReadonlyReads48 {\n@IsTest static void asyncErrorEqualityIsRead() {\n AsyncApexJob job = (AsyncApexJob)JSON.deserialize('{\"NumberOfErrors\":0}', AsyncApexJob.class); Integer count = 0;\n if (job.NumberOfErrors == 0) { count++; }\n System.assertEquals(1, count);\n job = (AsyncApexJob)JSON.deserialize('{\"NumberOfErrors\":2}', AsyncApexJob.class);\n if (job.NumberOfErrors == 0) { count++; }\n System.assertEquals(1, count); System.assertEquals(2, job.NumberOfErrors);\n}\n@IsTest static void assigneeEqualityIsRead() {\n Id userId = UserInfo.getUserId();\n PermissionSetAssignment assignment = (PermissionSetAssignment)JSON.deserialize('{\"AssigneeId\":\"' + userId + '\"}', PermissionSetAssignment.class);\n Integer count = 0; if (assignment.AssigneeId == userId) { count++; }\n System.assertEquals(1, count);\n assignment = new PermissionSetAssignment(); if (assignment.AssigneeId == userId) { count++; }\n System.assertEquals(1, count);\n}\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeReadonlyReads48.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>48.0</apiVersion><status>Active</status></ApexClass>\n"
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
      "passed": 2,
      "total": 2
    }
  }
}
`},
		{name: "readonly-reads-api63", fixture: `{
  "name": "readonly-reads-api63",
  "apiVersion": "63.0",
  "project": {
    "sourceApiVersion": "63.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeReadonlyReads63.cls",
      "content": "@IsTest private class GladeReadonlyReads63 {\n@IsTest static void organizationDateEqualityIsRead() {\n Organization organization = (Organization)JSON.deserialize('{\"IsSandbox\":false,\"TrialExpirationDate\":null}', Organization.class);\n Boolean isTestMode = false;\n Boolean enabled = isTestMode || (!organization.IsSandbox && organization.TrialExpirationDate == null);\n System.assertEquals(true, enabled);\n organization = (Organization)JSON.deserialize('{\"IsSandbox\":false,\"TrialExpirationDate\":\"2025-01-01T00:00:00.000Z\"}', Organization.class);\n enabled = isTestMode || (!organization.IsSandbox && organization.TrialExpirationDate == null);\n System.assertEquals(false, enabled);\n}\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeReadonlyReads63.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>\n"
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
		{name: "permission-writes-api44", fixture: `{
  "name": "permission-writes-api44",
  "apiVersion": "44.0",
  "project": {
    "sourceApiVersion": "44.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladePermissionWrites44.cls",
      "content": "@IsTest private class GladePermissionWrites44 {\n @IsTest static void assignmentRetainsBothReferences() {\n  PermissionSetAssignment assignment = new PermissionSetAssignment(); Id userId = UserInfo.getUserId(); Id permissionId = '0PS000000000001';\n  assignment.AssigneeId = userId; assignment.PermissionSetId = permissionId;\n  System.assertEquals(userId, assignment.AssigneeId); System.assertEquals(permissionId, assignment.PermissionSetId);\n }\n @IsTest static void insertedAssignmentPersistsAndDeletes() {\n  PermissionSet permission = new PermissionSet(Name='GladeOwnedWrite44', Label='Glade Owned Write 44'); insert permission;\n  PermissionSetAssignment assignment = new PermissionSetAssignment(); assignment.AssigneeId = UserInfo.getUserId(); assignment.PermissionSetId = permission.Id;\n  insert assignment;\n  PermissionSetAssignment saved = [SELECT AssigneeId, PermissionSetId FROM PermissionSetAssignment WHERE Id = :assignment.Id];\n  System.assertEquals(UserInfo.getUserId(), saved.AssigneeId); System.assertEquals(permission.Id, saved.PermissionSetId);\n  Id assignmentId = assignment.Id; delete assignment;\n  System.assertEquals(0, [SELECT COUNT() FROM PermissionSetAssignment WHERE Id = :assignmentId]);\n }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladePermissionWrites44.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>44.0</apiVersion><status>Active</status></ApexClass>\n"
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
      "passed": 2,
      "total": 2
    }
  }
}
`},
		{name: "permission-writes-api48", fixture: `{
  "name": "permission-writes-api48",
  "apiVersion": "48.0",
  "project": {
    "sourceApiVersion": "48.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladePermissionWrites48.cls",
      "content": "@IsTest private class GladePermissionWrites48 {\n @IsTest static void assignmentRetainsBothReferences() {\n  PermissionSetAssignment assignment = new PermissionSetAssignment(); Id userId = UserInfo.getUserId(); Id permissionId = '0PS000000000001';\n  assignment.AssigneeId = userId; assignment.PermissionSetId = permissionId;\n  System.assertEquals(userId, assignment.AssigneeId); System.assertEquals(permissionId, assignment.PermissionSetId);\n }\n @IsTest static void insertedAssignmentPersistsAndDeletes() {\n  PermissionSet permission = new PermissionSet(Name='GladeOwnedWrite48', Label='Glade Owned Write 48'); insert permission;\n  PermissionSetAssignment assignment = new PermissionSetAssignment(); assignment.AssigneeId = UserInfo.getUserId(); assignment.PermissionSetId = permission.Id;\n  insert assignment;\n  PermissionSetAssignment saved = [SELECT AssigneeId, PermissionSetId FROM PermissionSetAssignment WHERE Id = :assignment.Id];\n  System.assertEquals(UserInfo.getUserId(), saved.AssigneeId); System.assertEquals(permission.Id, saved.PermissionSetId);\n  Id assignmentId = assignment.Id; delete assignment;\n  System.assertEquals(0, [SELECT COUNT() FROM PermissionSetAssignment WHERE Id = :assignmentId]);\n }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladePermissionWrites48.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>48.0</apiVersion><status>Active</status></ApexClass>\n"
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
      "passed": 2,
      "total": 2
    }
  }
}
`},
		{name: "custom-orderitem-api36", fixture: `{
  "name": "custom-orderitem-api36",
  "apiVersion": "36.0",
  "project": {
    "sourceApiVersion": "36.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeReadonlyWrapper36.cls",
      "content": "global virtual class GladeReadonlyWrapper36 {}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeReadonlyWrapper36.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>36.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/OrderItem.cls",
      "content": "global virtual with sharing class OrderItem extends GladeReadonlyWrapper36 {\n private Id stored; public Integer writes = 0;\n public Id OrderId { get { return stored; } set { stored = value; writes++; } }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/OrderItem.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>36.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeCustomOrderItem36.cls",
      "content": "@IsTest private class GladeCustomOrderItem36 {\n @IsTest static void customSetterRetainsId() {\n  OrderItem testOrderItem = new OrderItem(); Id testOrderId = '801000000000001';\n  testOrderItem.OrderId = testOrderId;\n  System.assertEquals(testOrderId, testOrderItem.OrderId); System.assertEquals(1, testOrderItem.writes);\n }\n @IsTest static void customSetterRunsOnBothWrites() {\n  OrderItem testOrderItem = new OrderItem(); testOrderItem.OrderId = '801000000000001'; testOrderItem.OrderId = '801000000000002';\n  System.assertEquals((Id)'801000000000002', testOrderItem.OrderId); System.assertEquals(2, testOrderItem.writes);\n }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeCustomOrderItem36.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>36.0</apiVersion><status>Active</status></ApexClass>\n"
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
      "passed": 2,
      "total": 2
    }
  }
}
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var fixture struct {
				Source  []struct{ Path, Content string }
				Project json.RawMessage
			}
			if err := json.Unmarshal([]byte(tc.fixture), &fixture); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			for _, source := range fixture.Source {
				path := filepath.Join(root, filepath.FromSlash(source.Path))
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				writeSemaFile(t, path, source.Content)
			}
			writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), string(fixture.Project))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(typesys.Build(p, schema.Schema{}))
			if result.HasErrors() {
				t.Fatalf("Salesforce-admitted field/property access rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestReadonlyCreatedDateAssignmentRemainsRejected(t *testing.T) {
	// Exact API65 field-assignment shape from the admitted negative control.
	body := `Contact value=new Contact(); value.CreatedDate=Datetime.now();`
	check := func(t *testing.T, result Result) {
		t.Helper()
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA027" || result.Diagnostics[0].Message != `method "run" accesses field "value.CreatedDate" incorrectly: field is not writeable` {
			t.Fatalf("expected precise CreatedDate assignment rejection, got %#v", result.Diagnostics)
		}
	}
	root := t.TempDir()
	path := filepath.Join(root, "OwnedReadonlyNegative.cls")
	writeSemaFile(t, path, "public class OwnedReadonlyNegative { public static void run() {"+body+"} }")
	check(t, Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: "65.0", ApexFiles: []string{path}}, schema.Schema{})))
}
