trigger MissingPrefixKnownNestedTypeProbeTrigger on Account (before insert) {
    missingns.KnownType.Instance.x();
}
