public without sharing class IspSeeder {
    public static void seed(List<SObject> rows) {
        Database.insert(rows, true, AccessLevel.SYSTEM_MODE);
    }
}
