public class FernCasePB { @InvocableMethod(label='PB') public static void run(List<Case> rows) { FernCaseLog.events.add('pb:'+rows.size()); } }
