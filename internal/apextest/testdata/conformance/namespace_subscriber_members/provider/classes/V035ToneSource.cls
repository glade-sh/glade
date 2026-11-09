global class V035ToneSource implements V035Tone {
    global String blend(V035Env__c packet) { return 'provider'; }
    global void scatter(V035Sliver__c packet) {}
    global String echo(String packet) { return 'provider-text'; }
}
