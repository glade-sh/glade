trigger BulkReferenceTrigger on Account (before insert, before update) { BulkReferenceProbe.apply(Trigger.new); }
