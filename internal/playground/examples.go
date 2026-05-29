package playground

import "sort"

type exampleTemplate struct {
	ExampleProject
	Files map[string]string
}

func ListExampleProjects() []ExampleProject {
	out := make([]ExampleProject, 0, len(exampleProjects))
	for _, example := range exampleProjects {
		meta := example.ExampleProject
		meta.Source = "builtin"
		meta.FileCount = len(example.Files)
		meta.Tags = append([]string(nil), meta.Tags...)
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func exampleTemplateByID(id string) (exampleTemplate, bool) {
	for _, example := range exampleProjects {
		if example.ID == id {
			return example, true
		}
	}
	return exampleTemplate{}, false
}

var exampleProjects = []exampleTemplate{
	{
		ExampleProject: ExampleProject{
			ID:          "account-service",
			Name:        "Account Factory + Selector",
			Description: "One service inserts an Account, one reporter formats it, and SOQL reads back the saved fields.",
			Tags:        []string{"DML", "SOQL", "classes"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/AccountService.cls": `public class AccountService {
  public static Account createAccount(String name, String number) {
    Account account = new Account(Name = name, AccountNumber = number);
    insert account;
    return [
      SELECT Id, Name, AccountNumber
      FROM Account
      WHERE Id = :account.Id
      LIMIT 1
    ];
  }
}
`,
			"force-app/main/default/classes/AccountReporter.cls": `public class AccountReporter {
  public static String label(Account account) {
    return account.Name + ' #' + account.AccountNumber;
  }
}
`,
			"anonymous.apex": `Account account = AccountService.createAccount('North Ridge Supply', 'A-100');
System.debug(AccountReporter.label(account));
System.debug(Limits.getDmlStatements());
`,
			"seed.json": "{}\n",
		},
	},
	{
		ExampleProject: ExampleProject{
			ID:          "trigger-contact-task",
			Name:        "Before Insert Trigger",
			Description: "An Account trigger fills AccountNumber during insert, then the anonymous script queries the stamped row.",
			Tags:        []string{"triggers", "DML", "SOQL"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/TriggerExample.cls": `public class TriggerExample {
  public static Account run(String name) {
    Account account = new Account(Name = name);
    insert account;
    return [
      SELECT Id, Name, AccountNumber
      FROM Account
      WHERE Id = :account.Id
      LIMIT 1
    ];
  }
}
`,
			"force-app/main/default/triggers/AccountTaskTrigger.trigger": `trigger AccountTaskTrigger on Account (before insert) {
  for (Account account : Trigger.new) {
    if (account.AccountNumber == null) {
      account.AccountNumber = 'AUTO-' + account.Name;
    }
  }
}
`,
			"anonymous.apex": `Account account = TriggerExample.run('North Ridge');
System.debug(account.AccountNumber);
`,
			"seed.json": "{}\n",
		},
	},
	{
		ExampleProject: ExampleProject{
			ID:          "collection-selector",
			Name:        "Collection Selector",
			Description: "List DML seeds several Accounts, then selector and report classes filter Energy accounts.",
			Tags:        []string{"collections", "SOQL", "reports"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/AccountSeeder.cls": `public class AccountSeeder {
  public static void seed() {
    List<Account> accounts = new List<Account>();
    accounts.add(new Account(Name = 'Acme Energy', Industry = 'Energy'));
    accounts.add(new Account(Name = 'Blue Pine Health', Industry = 'Healthcare'));
    accounts.add(new Account(Name = 'Cinder Works', Industry = 'Energy'));
    insert accounts;
  }
}
`,
			"force-app/main/default/classes/IndustrySelector.cls": `public class IndustrySelector {
  public static List<Account> byIndustry(String industry) {
    return [
      SELECT Id, Name, Industry
      FROM Account
      WHERE Industry = :industry
    ];
  }
}
`,
			"force-app/main/default/classes/IndustryReport.cls": `public class IndustryReport {
  public static String summarize(String industry) {
    List<Account> accounts = IndustrySelector.byIndustry(industry);
    return industry + ': ' + accounts.size();
  }
}
`,
			"anonymous.apex": `AccountSeeder.seed();
System.debug(IndustryReport.summarize('Energy'));
for (Account account : IndustrySelector.byIndustry('Energy')) {
  System.debug(account.Name);
}
`,
			"seed.json": "{}\n",
		},
	},
	{
		ExampleProject: ExampleProject{
			ID:          "persist-mode-ledger",
			Name:        "Persist Mode Ledger",
			Description: "Flip Advanced to persist mode and run twice to see Accounts accumulate between executions.",
			Tags:        []string{"persist", "org diff", "SOQL"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/LedgerService.cls": `public class LedgerService {
  public static Integer addEntry(String name) {
    insert new Account(Name = name);
    return [
      SELECT Id, Name
      FROM Account
    ].size();
  }
}
`,
			"anonymous.apex": `Integer total = LedgerService.addEntry('Run ' + String.valueOf(DateTime.now().getTime()));
System.debug('total accounts: ' + total);
`,
			"seed.json": "{}\n",
		},
	},
	{
		ExampleProject: ExampleProject{
			ID:          "bulk-trigger-rollup",
			Name:        "Bulk Trigger Rollup",
			Description: "Bulk Account insert invokes a trigger for every row and prints the assigned auto numbers.",
			Tags:        []string{"bulk", "triggers", "limits"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/AutoNumberService.cls": `public class AutoNumberService {
  public static String next(Integer value) {
    return 'AUTO-' + String.valueOf(value);
  }
}
`,
			"force-app/main/default/classes/BulkTriggerScenario.cls": `public class BulkTriggerScenario {
  public static List<Account> create(Integer count) {
    List<Account> accounts = new List<Account>();
    for (Integer i = 1; i <= count; i++) {
      accounts.add(new Account(Name = 'Bulk ' + String.valueOf(i), Industry = 'Energy'));
    }
    insert accounts;
    return [
      SELECT Id, Name, AccountNumber, Industry
      FROM Account
      WHERE Industry = 'Energy'
    ];
  }
}
`,
			"force-app/main/default/triggers/AccountAutoNumberTrigger.trigger": `trigger AccountAutoNumberTrigger on Account (before insert) {
  Integer index = 0;
  for (Account account : Trigger.new) {
    index++;
    account.AccountNumber = AutoNumberService.next(index);
  }
}
`,
			"anonymous.apex": `List<Account> accounts = BulkTriggerScenario.create(3);
System.debug('created: ' + accounts.size());
for (Account account : accounts) {
  System.debug(account.Name + ' ' + account.AccountNumber);
}
System.debug('dml rows: ' + Limits.getDmlRows());
`,
			"seed.json": "{}\n",
		},
	},
	{
		ExampleProject: ExampleProject{
			ID:          "map-selector-drill",
			Name:        "Map Selector Drill",
			Description: "SOQL rows are folded into a Map<String,Integer> so the script can read counts by industry.",
			Tags:        []string{"maps", "SOQL", "collections"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/MapSelectorDrill.cls": `public class MapSelectorDrill {
  public static void seed() {
    List<Account> accounts = new List<Account>();
    accounts.add(new Account(Name = 'Acme Energy', Industry = 'Energy'));
    accounts.add(new Account(Name = 'Cinder Energy', Industry = 'Energy'));
    accounts.add(new Account(Name = 'Blue Pine Health', Industry = 'Healthcare'));
    insert accounts;
  }

  public static Map<String, Integer> countByIndustry() {
    Map<String, Integer> counts = new Map<String, Integer>();
    for (Account account : [
      SELECT Id, Name, Industry
      FROM Account
    ]) {
      Integer current = counts.get(account.Industry);
      if (current == null) {
        current = 0;
      }
      counts.put(account.Industry, current + 1);
    }
    return counts;
  }
}
`,
			"anonymous.apex": `MapSelectorDrill.seed();
Map<String, Integer> counts = MapSelectorDrill.countByIndustry();
System.debug('Energy => ' + counts.get('Energy'));
System.debug('Healthcare => ' + counts.get('Healthcare'));
`,
			"seed.json": "{}\n",
		},
	},
	{
		ExampleProject: ExampleProject{
			ID:          "contact-relationship-drill",
			Name:        "Account + Contact Query",
			Description: "Creates Contacts under one Account and queries them back by AccountId.",
			Tags:        []string{"relationships", "contacts", "SOQL"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/ContactRelationshipDrill.cls": `public class ContactRelationshipDrill {
  public static Id seed() {
    Account account = new Account(Name = 'Relationship Test');
    insert account;
    List<Contact> contacts = new List<Contact>();
    contacts.add(new Contact(LastName = 'One', AccountId = account.Id));
    contacts.add(new Contact(LastName = 'Two', AccountId = account.Id));
    contacts.add(new Contact(LastName = 'Three', AccountId = account.Id));
    insert contacts;
    return account.Id;
  }

  public static Integer countFor(Id accountId) {
    return [
      SELECT Id, LastName, AccountId
      FROM Contact
      WHERE AccountId = :accountId
    ].size();
  }
}
`,
			"anonymous.apex": `Id accountId = ContactRelationshipDrill.seed();
Integer total = ContactRelationshipDrill.countFor(accountId);
System.debug('contacts: ' + total);
System.debug('queries: ' + Limits.getQueries());
`,
			"seed.json": "{}\n",
		},
	},
	{
		ExampleProject: ExampleProject{
			ID:          "governor-limits-strict",
			Name:        "Governor Limits (strict)",
			Description: "Flip Advanced to strict to make governor counters enforce hard limits; permissive mode prints the same counters.",
			Tags:        []string{"limits", "strict", "SOQL"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/StrictLimitShowcase.cls": `public class StrictLimitShowcase {
  public static void seed(Integer count) {
    List<Account> accounts = new List<Account>();
    for (Integer i = 1; i <= count; i++) {
      accounts.add(new Account(Name = 'Strict ' + String.valueOf(i)));
    }
    insert accounts;
  }

  public static Integer countAccounts() {
    return [
      SELECT Id, Name
      FROM Account
    ].size();
  }
}
`,
			"anonymous.apex": `StrictLimitShowcase.seed(3);
Integer first = StrictLimitShowcase.countAccounts();
Integer second = StrictLimitShowcase.countAccounts();
System.debug('counts: ' + first + '/' + second);
System.debug('queries used: ' + Limits.getQueries());
System.debug('dml rows used: ' + Limits.getDmlRows());
`,
			"seed.json": "{}\n",
		},
	},
	{
		ExampleProject: ExampleProject{
			ID:          "org-diff-dml",
			Name:        "Org Diff after DML",
			Description: "Inserts an Account, updates it, and leaves a clear inserted/updated footprint in Org diff.",
			Tags:        []string{"org diff", "DML", "updates"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/OrgDiffShowcase.cls": `public class OrgDiffShowcase {
  public static Account run() {
    Account account = new Account(Name = 'Diff Seed', Industry = 'Energy');
    insert account;
    account.Industry = 'Manufacturing';
    update account;
    return [
      SELECT Id, Name, Industry
      FROM Account
      WHERE Id = :account.Id
      LIMIT 1
    ];
  }
}
`,
			"anonymous.apex": `Account account = OrgDiffShowcase.run();
System.debug(account.Name + ' => ' + account.Industry);
System.debug('dml statements: ' + Limits.getDmlStatements());
System.debug('dml rows: ' + Limits.getDmlRows());
`,
			"seed.json": "{}\n",
		},
	},
	{
		ExampleProject: ExampleProject{
			ID:          "limit-counter-drill",
			Name:        "Governor Counter Drill",
			Description: "Runs DML plus repeated SOQL, then prints Limits counters that appear in the Results panel.",
			Tags:        []string{"limits", "DML", "SOQL"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/LimitCounterDrill.cls": `public class LimitCounterDrill {
  public static void insertBatch(Integer count) {
    List<Account> accounts = new List<Account>();
    for (Integer i = 1; i <= count; i++) {
      accounts.add(new Account(Name = 'Limit ' + String.valueOf(i), Industry = 'Manufacturing'));
    }
    insert accounts;
  }

  public static Integer countAccounts() {
    return [
      SELECT Id, Name
      FROM Account
    ].size();
  }
}
`,
			"anonymous.apex": `LimitCounterDrill.insertBatch(5);
Integer first = LimitCounterDrill.countAccounts();
Integer second = LimitCounterDrill.countAccounts();
System.debug('accounts: ' + first + '/' + second);
System.debug('dml rows: ' + Limits.getDmlRows());
System.debug('queries: ' + Limits.getQueries());
`,
			"seed.json": "{}\n",
		},
	},
}

const sfdxProjectJSON = `{"packageDirectories":[{"path":"force-app","default":true}],"name":"glade-playground-example","namespace":"","sourceApiVersion":"65.0"}` + "\n"
