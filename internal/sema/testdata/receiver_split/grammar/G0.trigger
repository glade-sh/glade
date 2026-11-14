trigger CrTrigG0 on Account (before insert) {
System.debug(Trigger.new);
CrProbe.value();
}
