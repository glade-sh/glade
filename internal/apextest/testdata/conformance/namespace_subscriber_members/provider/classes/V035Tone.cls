global interface V035Tone {
    String blend(V035Env__c packet);
    void scatter(V035Sliver__c packet);
    String echo(String packet);
}
