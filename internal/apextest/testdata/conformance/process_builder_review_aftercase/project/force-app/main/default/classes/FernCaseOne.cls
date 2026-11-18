public class FernCaseOne { @InvocableMethod(label='One') public static void run(List<Id> caseIds) { FernCaseLog.events.add('one:'+caseIds.size()); } }
