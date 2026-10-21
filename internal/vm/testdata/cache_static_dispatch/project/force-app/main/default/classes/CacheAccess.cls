public class CacheAccess {
    public static Cache.OrgPartition organization(){return Cache.Org.getPartition('local.default');}
    public static Cache.SessionPartition session(){return Cache.Session.getPartition('local.default');}
    public static Boolean available(){return Cache.Session.isAvailable();}
}
