public class ShapeUse {
    public Account fieldAccount = new Account('Account');
    public Note fieldNote = new Note('Note');
    public JWT fieldJWT = new JWT('JWT');
    public String topLevelRoute() {
        Account localAccount = new Account('Account');
        Note localNote = new Note('Note');
        JWT localJWT = new JWT('JWT');
        return localAccount.topValue + ':' + localNote.topValue + ':' + localJWT.topValue + ':' + fieldAccount.topValue + ':' + fieldNote.topValue + ':' + fieldJWT.topValue;
    }
}
