public class NoteContainer {
    public Note note;

    public NoteContainer() {
        note = new Note('field');
    }

    public String fieldValue() {
        return note.marker;
    }
}
