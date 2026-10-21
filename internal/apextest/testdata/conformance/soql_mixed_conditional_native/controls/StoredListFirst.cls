public class StoredListFirst {
    public static Account choose(Boolean flag, List<Account> records, Account record) {
        Account chosen = flag ? records : record;
        return chosen;
    }
}
