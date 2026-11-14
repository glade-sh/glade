trigger RouteTrigTry on Account (before insert) {
try { CrProbe.
value(); } catch (Exception e) { }
}
