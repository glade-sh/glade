public virtual class BaseController {
    public class Note {
        public String sfObject;

        public Note(String value) {
            sfObject = 'inner:' + value;
        }
    }

    public class Contact {
        public String sfObject;

        public Contact(String sfObject) {
            this.sfObject = sfObject;
        }
    }
}
