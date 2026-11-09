public class ToneRelay {
    private gladens004.V035Tone held;
    public ToneRelay(gladens004.V035Tone c) { held = c; }
    private gladens004.V035Tone exposed() { return held; }
    public String relayEnvelope(gladens004__V035Env__c packet) { return exposed().blend(packet); }
    public void relaySliver(gladens004__V035Sliver__c packet) { exposed().scatter(packet); }
    public String relayText(String packet) { return exposed().echo(packet); }
}
