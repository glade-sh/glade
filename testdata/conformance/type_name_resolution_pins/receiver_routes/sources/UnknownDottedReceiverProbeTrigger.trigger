trigger UnknownDottedReceiverProbeTrigger on Account (before insert) {
    a.b.c.x();
}
