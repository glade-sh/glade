public class EditKindsUserTwo {
  public Integer run() {
    EditKindsSubject subject = new EditKindsSubject();
    String text = subject.label;
    EditKindsUserOne other = new EditKindsUserOne();
    return text == null ? 0 : other.run().length();
  }
}
