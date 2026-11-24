public class EditKindsSubject {
  public static Integer boot;
  private String stored;

  static {
    boot = 1;
  }

  public String label {
    get {
      return stored;
    }
    set {
      stored = value;
    }
  }

  public Integer total(Integer a, Integer b) {
    return a + b;
  }

  public class Marker {
    public String tag(String text) {
      return 'm:' + text;
    }
  }

  public List<Account> load() {
    return [SELECT Id, Name, Industry, Phone FROM Account LIMIT 10];
  }

  // The same field list as load(), so trimming one list leaves the hydrated
  // field set of the class unchanged.
  public List<Account> reload() {
    return [SELECT Id, Name, Industry, Phone FROM Account LIMIT 5];
  }
}
