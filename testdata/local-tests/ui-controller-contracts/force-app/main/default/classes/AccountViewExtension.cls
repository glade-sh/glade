public class AccountViewExtension {
  public String name;

  public AccountViewExtension(ApexPages.StandardController controller) {
    Account account = (Account) controller.getRecord();
    name = account.Name;
  }
}
