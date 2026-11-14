trigger RouteTrigForeachFree on Account (before insert) {
for (Account a : Trigger
.new) { }
}
