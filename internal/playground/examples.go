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
			ID:          "refinement-service",
			Name:        "Refinement Service",
			Description: "One service inserts a local row, one formatter labels it, and SOQL reads back the saved fields.",
			Tags:        []string{"DML", "SOQL", "classes"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/RefinementService.cls": `public class RefinementService {
  public static Account createFileRow(String name, String number) {
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
			"force-app/main/default/classes/FileRow.cls": `public class FileRow {
  public static String label(Account account) {
    return account.Name + ' #' + account.AccountNumber;
  }
}
`,
			"anonymous.apex": `Account row = RefinementService.createFileRow('Refine 01', 'F-100');
System.debug(FileRow.label(row));
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
			ID:          "deal-desk-discount-guard",
			Name:        "Deal Desk Discount Guard",
			Description: "Accounts flow through a trigger, selector, policy, and report before printing deal desk counters.",
			Tags:        []string{"DML", "SOQL", "triggers", "limits"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/AccountDealScenario.cls": `public class AccountDealScenario {
  public static List<Account> seed() {
    List<Account> accounts = new List<Account>();
    accounts.add(new Account(Name = 'North Ridge Manufacturing', Industry = 'Manufacturing', AnnualRevenue = 2500000));
    accounts.add(new Account(Name = 'Twin Lakes Energy', Industry = 'Energy', AnnualRevenue = 900000));
    accounts.add(new Account(Name = 'Port Alsworth Hardware', Industry = 'Retail', AnnualRevenue = 180000));
    insert accounts;
    return DealAccountSelector.allDealAccounts();
  }
}
`,
			"force-app/main/default/classes/DiscountPolicy.cls": `public class DiscountPolicy {
  public static String bucket(Account account) {
    if (account.AnnualRevenue >= 1000000) {
      return 'strategic';
    }
    if (account.Industry == 'Energy') {
      return 'managed';
    }
    return 'standard';
  }
}
`,
			"force-app/main/default/classes/DealAccountSelector.cls": `public class DealAccountSelector {
  public static List<Account> allDealAccounts() {
    return [
      SELECT Id, Name, Industry, AnnualRevenue, AccountNumber
      FROM Account
    ];
  }
}
`,
			"force-app/main/default/classes/DealDeskReport.cls": `public class DealDeskReport {
  public static void print() {
    List<Account> accounts = AccountDealScenario.seed();
    String topBucket = 'standard';
    String rowName = '';
    for (Account account : accounts) {
      String bucket = DiscountPolicy.bucket(account);
      if (bucket == 'strategic') {
        topBucket = bucket;
        rowName = account.Name;
      }
    }
    System.debug('deal count: ' + accounts.size());
    System.debug('top bucket: ' + topBucket);
    System.debug('dml rows: ' + Limits.getDmlRows());
    System.debug('queries: ' + Limits.getQueries());
    System.debug('row: ' + rowName);
  }
}
`,
			"force-app/main/default/triggers/AccountDealTrigger.trigger": `trigger AccountDealTrigger on Account (before insert) {
  Integer counter = 1;
  for (Account account : Trigger.new) {
    account.AccountNumber = 'DEAL-' + String.valueOf(counter);
    counter++;
  }
}
`,
			"anonymous.apex": `DealDeskReport.print();
`,
			"seed.json": "{}\n",
		},
	},
	{
		ExampleProject: ExampleProject{
			ID:          "renewal-health-scorecard",
			Name:        "Renewal Health Scorecard",
			Description: "A seeder, selector, rules class, and scorecard compute renewal health from Account and Contact rows.",
			Tags:        []string{"DML", "SOQL", "contacts", "limits"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/RenewalSeeder.cls": `public class RenewalSeeder {
  public static Id seed() {
    Account account = new Account(Name = 'Twin Lakes Renewals', Industry = 'Technology');
    insert account;
    List<Contact> contacts = new List<Contact>();
    contacts.add(new Contact(LastName = 'Owner', AccountId = account.Id));
    contacts.add(new Contact(LastName = 'Champion', AccountId = account.Id));
    contacts.add(new Contact(LastName = 'Support', AccountId = account.Id));
    insert contacts;
    return account.Id;
  }
}
`,
			"force-app/main/default/classes/RenewalAccountSelector.cls": `public class RenewalAccountSelector {
  public static Account accountById(Id accountId) {
    return [
      SELECT Id, Name, Industry
      FROM Account
      WHERE Id = :accountId
      LIMIT 1
    ];
  }

  public static List<Contact> contactsFor(Id accountId) {
    return [
      SELECT Id, LastName, AccountId
      FROM Contact
      WHERE AccountId = :accountId
    ];
  }
}
`,
			"force-app/main/default/classes/RenewalHealthRules.cls": `public class RenewalHealthRules {
  public static Integer score(List<Contact> contacts) {
    Integer score = 55 + (contacts.size() * 10);
    if (score > 100) {
      return 100;
    }
    return score;
  }
}
`,
			"force-app/main/default/classes/RenewalScorecard.cls": `public class RenewalScorecard {
  public static void print() {
    Id accountId = RenewalSeeder.seed();
    Account account = RenewalAccountSelector.accountById(accountId);
    List<Contact> contacts = RenewalAccountSelector.contactsFor(accountId);
    Integer score = RenewalHealthRules.score(contacts);
    System.debug('health score: ' + score);
    System.debug('contacts: ' + contacts.size());
    System.debug('queries: ' + Limits.getQueries());
    System.debug('row: ' + account.Name);
  }
}
`,
			"anonymous.apex": `RenewalScorecard.print();
`,
			"seed.json": "{}\n",
		},
	},
	{
		ExampleProject: ExampleProject{
			ID:          "org-diff-review-loop",
			Name:        "Org Diff Review Loop",
			Description: "A review class inserts an Account, a decision class updates it, and a report prints the final row.",
			Tags:        []string{"DML", "SOQL", "updates", "org diff"},
		},
		Files: map[string]string{
			"sfdx-project.json": sfdxProjectJSON,
			"force-app/main/default/classes/ReviewScenario.cls": `public class ReviewScenario {
  public static Id seed() {
    Account account = new Account(Name = 'Review Seed', Industry = 'Energy');
    insert account;
    return account.Id;
  }
}
`,
			"force-app/main/default/classes/ReviewSelector.cls": `public class ReviewSelector {
  public static Account byId(Id accountId) {
    return [
      SELECT Id, Name, Industry, AccountNumber
      FROM Account
      WHERE Id = :accountId
      LIMIT 1
    ];
  }
}
`,
			"force-app/main/default/classes/ReviewDecision.cls": `public class ReviewDecision {
  public static String approve(Id accountId) {
    Account account = ReviewSelector.byId(accountId);
    account.Industry = 'Manufacturing';
    account.AccountNumber = 'APPROVED';
    update account;
    return 'approved';
  }
}
`,
			"force-app/main/default/classes/ReviewReport.cls": `public class ReviewReport {
  public static void print() {
    Id accountId = ReviewScenario.seed();
    String decision = ReviewDecision.approve(accountId);
    Account account = ReviewSelector.byId(accountId);
    System.debug('decision: ' + decision);
    System.debug('row: ' + account.Name + ' / ' + account.Industry);
    System.debug('dml rows: ' + Limits.getDmlRows());
  }
}
`,
			"anonymous.apex": `ReviewReport.print();
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
