public class FaultScalarProbe {
    public static String sample() {
        Contact row = new Contact(LastName = 'Synthetic notification');
        insert as system row;
        Integer saved = [SELECT COUNT() FROM Contact WHERE Id = :row.Id WITH SYSTEM_MODE];
        delete as system row;
        return JSON.serialize(new Map<String,Object>{'saved' => saved});
    }
}
