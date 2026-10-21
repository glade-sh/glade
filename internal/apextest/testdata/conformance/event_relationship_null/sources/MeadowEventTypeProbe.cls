public class MeadowEventTypeProbe {
    public static Map<String, Object> observe() {
        Savepoint boundary = Database.setSavepoint();
        try {
            Account parent = new Account(Name = 'Meadow Parent');
            insert parent;
            Opportunity proposal = new Opportunity(Name = 'Meadow Proposal', AccountId = parent.Id,
                StageName = 'Prospecting', CloseDate = Date.today().addDays(7));
            insert proposal;
            Datetime begins = Datetime.now().addHours(1);
            List<Event> meetings = new List<Event>{
                new Event(Subject = 'Meadow Unlinked', ShowAs = 'Busy', StartDateTime = begins, EndDateTime = begins.addHours(1)),
                new Event(Subject = 'Meadow Account', WhatId = parent.Id, ShowAs = 'Busy', StartDateTime = begins, EndDateTime = begins.addHours(1)),
                new Event(Subject = 'Meadow Opportunity', WhatId = proposal.Id, ShowAs = 'Busy', StartDateTime = begins, EndDateTime = begins.addHours(1))
            };
            insert meetings;
            Id unlinkedId = meetings[0].Id;
            Id accountId = meetings[1].Id;
            Id opportunityId = meetings[2].Id;
            Set<Id> meetingIds = new Set<Id>{unlinkedId, accountId, opportunityId};
            Datetime windowStart = Datetime.now().addHours(-1);
            Map<String, Object> rows = new Map<String, Object>();
            rows.put('EN01', [SELECT COUNT() FROM Event WHERE Id = :unlinkedId AND What.Type != 'Account']);
            rows.put('EN02', [SELECT COUNT() FROM Event WHERE Id = :unlinkedId AND What.Type != 'Opportunity']);
            rows.put('EN03', [SELECT COUNT() FROM Event WHERE Id = :unlinkedId AND What.Type != 'Account' AND What.Type != 'Opportunity']);
            rows.put('EN04', [SELECT COUNT() FROM Event WHERE Id = :accountId AND What.Type != 'Account']);
            rows.put('EN05', [SELECT COUNT() FROM Event WHERE Id = :accountId AND What.Type != 'Opportunity']);
            rows.put('EN06', [SELECT COUNT() FROM Event WHERE Id = :opportunityId AND What.Type != 'Account']);
            rows.put('EN07', [SELECT COUNT() FROM Event WHERE Id = :opportunityId AND What.Type != 'Opportunity']);
            rows.put('EN08', [SELECT COUNT() FROM Event WHERE Id = :unlinkedId AND What.Type = null]);
            rows.put('EN09', Database.query('SELECT Id FROM Event WHERE Id = :unlinkedId AND What.Type != \'Account\' AND What.Type != \'Opportunity\' AND LastModifiedDate > :windowStart AND ShowAs != \'Free\'').size());
            rows.put('EC01', [SELECT COUNT() FROM Event WHERE Id IN :meetingIds]);
            rows.put('EC02', [SELECT WhatId FROM Event WHERE Id = :unlinkedId].WhatId == null);
            rows.put('EC03', [SELECT COUNT() FROM Event WHERE Id = :accountId AND What.Type = 'Account']);
            return rows;
        } finally {
            Database.rollback(boundary);
        }
    }
}
