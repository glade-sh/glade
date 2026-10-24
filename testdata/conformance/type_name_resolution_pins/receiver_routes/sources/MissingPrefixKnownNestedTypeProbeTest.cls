@IsTest private class MissingPrefixKnownNestedTypeProbeTest {
    @IsTest static void observe() {
        missingns.KnownType.Instance.x();
    }
}
