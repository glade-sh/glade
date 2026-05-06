public class EmailProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'email.single-message') {
            return runSingleMessage();
        } else if (probeId == 'email.limits') {
            return runLimits();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runSingleMessage() {
        Messaging.SingleEmailMessage mail = new Messaging.SingleEmailMessage();
        mail.setToAddresses(new String[] { 'test@example.com' });
        mail.setSubject('Probe Test');
        mail.setPlainTextBody('Hello from probe');
        return mail.getSubject();
    }

    private Object runLimits() {
        return Limits.getLimitEmailInvocations();
    }
}
