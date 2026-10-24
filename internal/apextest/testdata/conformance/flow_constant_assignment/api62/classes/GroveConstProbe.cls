public class GroveConstProbe {
    public static String observe() {
        Map<String,Object> result = new Map<String,Object>();
        for (String key : new List<String>{'K01','K02','K03','K04','K05'}) {
            Map<String,Object> cell = new Map<String,Object>{'description'=>null,'employees'=>null,'exception'=>null};
            try {
                Account row = new Account(Name=key);
                insert row;
                Account stored = [SELECT Description, NumberOfEmployees FROM Account WHERE Id=:row.Id];
                cell.put('description', stored.Description);
                cell.put('employees', stored.NumberOfEmployees);
            } catch (Exception error) { cell.put('exception', error.getTypeName()); }
            result.put(key, cell);
        }
        return JSON.serialize(result);
    }
}
