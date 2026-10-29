public class CountedFaultException extends DetailedFaultException {
    public CountedFaultException(Integer count) {
        detail = count;
    }
}
