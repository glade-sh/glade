public class AccountService {
  public Integer countAccounts(AccountSelector selector) {
    return selector.selectAll().size();
  }
}
