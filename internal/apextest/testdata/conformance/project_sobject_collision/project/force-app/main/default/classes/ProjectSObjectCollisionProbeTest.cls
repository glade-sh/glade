@IsTest
private class ProjectSObjectCollisionProbeTest {
    @IsTest
    static void accountVariable() {
        Account value = new Account('variable');
        observe('accountVariable', value.sfObject);
    }

    @IsTest
    static void accountField() {
        AccountContainer value = new AccountContainer();
        observe('accountField', value.fieldValue());
    }

    @IsTest
    static void noteVariable() {
        Note value = new Note('noteVariable');
        observe('noteVariable', value.marker);
    }

    @IsTest
    static void noteField() {
        NoteContainer value = new NoteContainer();
        observe('noteField', value.fieldValue());
    }

    @IsTest
    static void inheritedNoteVariable() {
        DerivedNoteController value = new DerivedNoteController();
        observe('inheritedNoteVariable', value.variableValue());
    }

    @IsTest
    static void inheritedNoteField() {
        DerivedNoteController value = new DerivedNoteController();
        observe('inheritedNoteField', value.fieldValue());
    }

    @IsTest
    static void inheritedContactVariable() {
        DerivedController value = new DerivedController();
        observe('inheritedContactVariable', value.variableValue());
    }

    @IsTest
    static void inheritedContactField() {
        DerivedController value = new DerivedController();
        observe('inheritedContactField', value.fieldValue());
    }

    @IsTest
    static void credentialShape() {
        CredentialShape value = new CredentialShape();
        observe('credentialShape', value.read());
    }

    private static void observe(String methodName, String value) {
        System.assert(false, 'OBSERVED|' + methodName + '|' + JSON.serialize(new Map<String, Object>{'value' => value}));
    }
}
