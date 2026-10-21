public class UpdateEntryProbe {
    public static String sample() {
        Account row = new Account(Name = 'Synthetic entry', Industry = 'Agriculture', Description = 'initial');
        insert as system row;
        row.Industry = 'Technology';
        update as system row;
        String other = [SELECT Description FROM Account WHERE Id = :row.Id WITH SYSTEM_MODE].Description;
        row.Industry = 'Banking';
        update as system row;
        String matched = [SELECT Description FROM Account WHERE Id = :row.Id WITH SYSTEM_MODE].Description;
        row.Description = 'unchanged industry';
        update as system row;
        String unchanged = [SELECT Description FROM Account WHERE Id = :row.Id WITH SYSTEM_MODE].Description;
        delete as system row;
        return JSON.serialize(new Map<String,Object>{'other' => other, 'matched' => matched, 'unchanged' => unchanged});
    }
}
