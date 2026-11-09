public class QuietTone implements gladens004.V035Tone {
    public Integer visits = 0;
    public String blend(gladens004__V035Env__c packet) { return 'subscriber'; }
    public void scatter(gladens004__V035Sliver__c packet) { visits++; }
    public String echo(String packet) { return 'subscriber-text'; }
}
