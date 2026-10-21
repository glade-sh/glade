public class AccountContainer {
    public Account account;

    public AccountContainer() {
        account = new Account('field');
    }

    public String fieldValue() {
        return account.sfObject;
    }
}
