public class ShapeDerived extends ShapeBase {
    public Account fieldAccount = new Account('Account');
    public Note fieldNote = new Note('Note');
    public Contact fieldContact = new Contact('Contact');
    public JWT fieldJWT = new JWT('JWT');
    public String inheritedLocalRoute() {
        Account localAccount = new Account('Account');
        Note localNote = new Note('Note');
        Contact localContact = new Contact('Contact');
        JWT localJWT = new JWT('JWT');
        return localAccount.nestedValue + ':' + localNote.nestedValue + ':' + localContact.nestedValue + ':' + localJWT.nestedValue;
    }
    public String inheritedFieldRoute() {
        return fieldAccount.nestedValue + ':' + fieldNote.nestedValue + ':' + fieldContact.nestedValue + ':' + fieldJWT.nestedValue;
    }
}
