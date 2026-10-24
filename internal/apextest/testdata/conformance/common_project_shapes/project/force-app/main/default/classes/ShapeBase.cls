public virtual class ShapeBase {
    public class Account {
        public String nestedValue;
        public Account(String value) { nestedValue = value; }
    }
    public class Note {
        public String nestedValue;
        public Note(String value) { nestedValue = value; }
    }
    public class Contact {
        public String nestedValue;
        public Contact(String value) { nestedValue = value; }
    }
    public class JWT {
        public String nestedValue;
        public JWT(String value) { nestedValue = value; }
    }
    public String nestedRoute() {
        Account localAccount = new Account('Account');
        Note localNote = new Note('Note');
        Contact localContact = new Contact('Contact');
        JWT localJWT = new JWT('JWT');
        return localAccount.nestedValue + ':' + localNote.nestedValue + ':' + localContact.nestedValue + ':' + localJWT.nestedValue;
    }
}
