trigger MissingPrefixKnownTypeProbeTrigger on Account (before insert) {
    missingns.KnownType.x();
}
