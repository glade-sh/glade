public class StoredListSecond {
    public static Account choose(Boolean flag, List<Account> records, Account record) {
        Account chosen = flag ? record : records;
        return chosen;
    }
}
