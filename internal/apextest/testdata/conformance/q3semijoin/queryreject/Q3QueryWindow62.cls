@IsTest private class Q3QueryWindow62 { @IsTest static void rejected() {
List<Account> rows = [SELECT Id, (SELECT Id FROM Contacts WHERE Id IN (SELECT ContactId FROM OpportunityContactRole LIMIT 1)) FROM Account];

} }
