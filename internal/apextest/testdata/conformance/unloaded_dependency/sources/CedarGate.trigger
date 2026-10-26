trigger CedarGate on Account (before insert) { CedarLibrary.apply(Trigger.new); }
