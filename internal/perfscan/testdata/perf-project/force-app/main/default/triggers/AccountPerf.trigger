trigger AccountPerf on Account (before insert, before update) {
    for (Account accountRecord : Trigger.new) {
        Account existingAccount = [SELECT Id, Name FROM Account WHERE Name = :accountRecord.Name LIMIT 1];
        accountRecord.Description = existingAccount.Name;
    }
}
