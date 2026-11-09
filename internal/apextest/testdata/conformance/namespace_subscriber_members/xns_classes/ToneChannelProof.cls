@IsTest
private class ToneChannelProof {
    private class InnerTone implements gladens004.V035Tone {
        public Integer visits = 0;
        public String blend(gladens004__V035Env__c packet) { return 'subscriber'; }
        public void scatter(gladens004__V035Sliver__c packet) { visits++; }
        public String echo(String packet) { return 'subscriber-text'; }
    }
    @IsTest static void subscriberObjectContractCall() { obs('X01', new ToneRelay(new InnerTone()).relayEnvelope(new gladens004__V035Env__c())); }
    @IsTest static void subscriberObjectVoidCall() { InnerTone q = new InnerTone(); new ToneRelay(q).relaySliver(new gladens004__V035Sliver__c()); obs('X02', String.valueOf(q.visits)); }
    @IsTest static void directSubscriberCall() { obs('X03', new InnerTone().blend(new gladens004__V035Env__c())); }
    @IsTest static void scalarContractCall() { obs('X04', new ToneRelay(new InnerTone()).relayText('leaf')); }
    @IsTest static void providerClassThroughRelay() { obs('X05', new ToneRelay(new gladens004.V035ToneSource()).relayEnvelope(new gladens004__V035Env__c())); }
    @IsTest static void providerClassText() { obs('X06', new ToneRelay(new gladens004.V035ToneSource()).relayText('leaf')); }
    @IsTest static void topLevelSubscriberImpl() { QuietTone q = new QuietTone(); new ToneRelay(q).relaySliver(new gladens004__V035Sliver__c()); obs('X07', new ToneRelay(q).relayEnvelope(new gladens004__V035Env__c()) + '/' + q.visits); }
    @IsTest static void insertedRecordDispatch() { gladens004__V035Env__c e = new gladens004__V035Env__c(Name='n'); insert e; obs('X08', new ToneRelay(new InnerTone()).relayEnvelope(e)); }
    static void obs(String id, String v) { System.assert(false, 'OBS|' + id + '|' + v); }
}
