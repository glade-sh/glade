public class BeChildBatch implements Database.Batchable<String> {
        String ctx;
        public BeChildBatch(String ctx) { this.ctx = ctx; }
        public Iterable<String> start(Database.BatchableContext c) { return new List<String>{'x'}; }
        public void execute(Database.BatchableContext c, List<String> scope) { BeProbe.record(ctx, null, true); }
        public void finish(Database.BatchableContext c) {}
    }
