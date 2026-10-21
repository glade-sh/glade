public class StandardIdentityProbe {
    public static String sample() {
        Account owner = new Account(Name = 'Synthetic identity');
        insert as system owner;
        Order purchase = new Order(AccountId = owner.Id, EffectiveDate = Date.today(), Status = 'Draft');
        insert as system purchase;
        Order stored = [SELECT OrderNumber FROM Order WHERE Id = :purchase.Id WITH SYSTEM_MODE];
        WorkOrder job = new WorkOrder(AccountId = owner.Id, Subject = 'Synthetic job');
        insert as system job;
        ServiceAppointment visit = new ServiceAppointment(ParentRecordId = job.Id, Subject = 'Synthetic visit', EarliestStartTime = System.now(), DueDate = System.now().addDays(2));
        insert as system visit;
        ServiceAppointment observed = [SELECT AccountId FROM ServiceAppointment WHERE Id = :visit.Id WITH SYSTEM_MODE];
        Map<String,Object> result = new Map<String,Object>{'orderNumberPresent' => String.isNotBlank(stored.OrderNumber), 'orderNumberDigits' => stored.OrderNumber != null && Pattern.matches('[0-9]{8}',stored.OrderNumber), 'visitAccount' => observed.AccountId == owner.Id};
        delete as system visit;
        delete as system job;
        delete as system purchase;
        delete as system owner;
        return JSON.serialize(result);
    }
}
