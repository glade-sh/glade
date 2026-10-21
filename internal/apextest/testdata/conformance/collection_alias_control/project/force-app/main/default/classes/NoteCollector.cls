public class NoteCollector {
    public class Note {
        public String text;
        public Note(String text) { this.text = text; }
    }
    public class Packet {
        public List<Note> notes { get; set; }
        public Integer observedSize { get; set; }
    }
    private final List<Note> notes = new List<Note>();
    public static Packet collect(Boolean assignFirst, Boolean useHelper) {
        return new NoteCollector().run(assignFirst, useHelper);
    }
    private Packet run(Boolean assignFirst, Boolean useHelper) {
        Packet packet = new Packet();
        if (assignFirst) packet.notes = this.notes;
        if (useHelper) {
            addOne();
        } else {
            this.notes.add(new Note('alpha'));
        }
        if (!assignFirst) packet.notes = this.notes;
        packet.observedSize = this.notes.size();
        return packet;
    }
    private void addOne() { this.notes.add(new Note('alpha')); }
}