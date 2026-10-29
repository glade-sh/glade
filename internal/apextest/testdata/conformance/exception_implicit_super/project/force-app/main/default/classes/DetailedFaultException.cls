public abstract class DetailedFaultException extends Exception {
    public Integer detail;
    public DetailedFaultException(String message, Integer detail) {
        this(message);
        this.detail = detail;
    }
}
