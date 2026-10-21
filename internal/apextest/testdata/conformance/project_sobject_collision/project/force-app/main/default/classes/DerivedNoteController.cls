public class DerivedNoteController extends BaseController {
    public Note note;

    public DerivedNoteController() {
        note = new Note('field');
    }

    public String variableValue() {
        Note local = new Note('variable');
        return local.sfObject;
    }

    public String fieldValue() {
        return note.sfObject;
    }
}
