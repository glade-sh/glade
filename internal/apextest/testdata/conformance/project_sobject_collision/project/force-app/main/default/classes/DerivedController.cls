public class DerivedController extends BaseController {
    public Contact contact;

    public DerivedController() {
        contact = new Contact('innerField');
    }

    public String variableValue() {
        Contact local = new Contact('innerVariable');
        return local.sfObject;
    }

    public String fieldValue() {
        return contact.sfObject;
    }
}
