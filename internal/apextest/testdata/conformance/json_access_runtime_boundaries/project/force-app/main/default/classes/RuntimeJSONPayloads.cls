public class RuntimeJSONPayloads {
    @JsonAccess(serializable='never' deserializable='never')
    public class Blocked {
        public String value = 'local';
    }
    @JsonAccess(serializable='never' deserializable='always')
    public class DecodeOnly { public String value; }
    @JsonAccess(serializable='always' deserializable='never')
    public class EncodeOnly { public String value = 'local'; }
}
