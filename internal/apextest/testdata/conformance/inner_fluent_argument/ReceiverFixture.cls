public class ReceiverFixture {
    public class Builder {
        public Builder accept(String value) { return this; }
        public String finish() { return 'ok'; }
    }
    public class Value {
        public String read() { return 'value'; }
    }
    public static Builder start() { return new Builder(); }
}
