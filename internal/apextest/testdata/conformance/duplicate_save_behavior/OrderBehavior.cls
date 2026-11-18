public class OrderBehavior {
    public static String overlap() {
        Contact seed=new Contact(LastName='Test1',Email='overlap@sample.example',Department='CustomBlock',Phone='4155551234');
        insert seed;
        StandardBehavior.remember(seed.Id,'seed');
        Contact other=new Contact(LastName='Test2',Email=seed.Email,Department='CustomBlock',Phone='4155551235');
        String output=StandardBehavior.find(other)+'|plain='+StandardBehavior.save(Database.insert(other,false));
        Database.DMLOptions settings=new Database.DMLOptions(); settings.DuplicateRuleHeader.allowSave=true;
        return output+'|override='+StandardBehavior.save(Database.insert(other,settings))+'|count='+[SELECT COUNT() FROM Contact WHERE Department='CustomBlock'];
    }
}
