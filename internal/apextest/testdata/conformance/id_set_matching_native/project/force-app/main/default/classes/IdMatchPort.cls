public interface IdMatchPort {
    void accept(Set<Id> values);
    void acceptToken(IdMatchToken value);
    void acceptAmount(Decimal value);
}
