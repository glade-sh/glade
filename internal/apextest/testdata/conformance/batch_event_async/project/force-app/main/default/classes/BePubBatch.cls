// Publisher batch: start or execute publishes one event whose trigger makes the call.
public class BePubBatch implements Database.Batchable<String> {
        String phase; String evt; String kind; Boolean deliver;
        public BePubBatch(String phase, String evt, String kind, Boolean deliver) { this.phase = phase; this.evt = evt; this.kind = kind; this.deliver = deliver; }
        void fire(String at) {
            Map<String, Object> extra = new Map<String, Object>();
            try { BeProbe.publish(evt, 'call:' + kind); extra.put('publish', 'ok'); } catch (Exception e) { extra.put('publish', BeProbe.err(e)); }
            if (deliver) {
                try { Test.getEventBus().deliver(); extra.put('deliver', 'ok'); } catch (Exception e) { extra.put('deliver', BeProbe.err(e)); }
            }
            extra.put('obsAfter', BeProbe.names());
            BeProbe.record('batch_' + at, extra, false);
        }
        public Iterable<String> start(Database.BatchableContext c) { if (phase == 'start') { fire('start'); } return new List<String>{'x'}; }
        public void execute(Database.BatchableContext c, List<String> scope) { if (phase == 'execute') { fire('execute'); } }
        public void finish(Database.BatchableContext c) {}
    }
