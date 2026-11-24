public class EditKindsUserOne {
  public String run() {
    EditKindsSubject subject = new EditKindsSubject();
    subject.label = 'one';
    EditKindsSubject.Marker marker = new EditKindsSubject.Marker();
    List<Account> rows = subject.load();
    Integer n = EditKindsSubject.boot + subject.total(1, 2) + rows.size();
    return marker.tag(subject.label) + n;
  }
}
