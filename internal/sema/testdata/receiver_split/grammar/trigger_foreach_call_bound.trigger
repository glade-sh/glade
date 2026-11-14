trigger RouteTrigForeachCall on Account (before insert) {
for (String s : CrProbe.
value().split(' ')) { }
}
