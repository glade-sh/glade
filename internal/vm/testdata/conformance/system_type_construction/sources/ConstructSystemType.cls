public class ConstructSystemType {
    public static Map<String,Object> observe() {
        Map<String,Object> rows = new Map<String,Object>();
        List<Type> types = new List<Type>{System.UserInfo.class, UserInfo.class, Type.forName('System','UserInfo'), Type.forName('UserInfo'), System.Limits.class, System.Finalizer.class, String.class, Account.class};
        List<String> names = new List<String>{'qualified','unqualified','namespace','lookup','limits','interface','string','record'};
        for (Integer i=0; i<types.size(); i++) {
            try { Object value=types[i].newInstance(); rows.put(names[i], value!=null); }
            catch(Exception e) { rows.put(names[i],e.getTypeName()+': '+e.getMessage()); }
        }
        return rows;
    }
}
