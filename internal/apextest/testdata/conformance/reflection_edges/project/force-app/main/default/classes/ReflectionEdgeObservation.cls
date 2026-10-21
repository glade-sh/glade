@IsTest private class ReflectionEdgeObservation {
    public class Parcel { public Datetime stamp; }
    @IsTest static void tokenText() {
        String observed = ReflectionEdgeHelper.describeType(List<Parcel>.class);
        System.assert(false, 'OBSERVED|tokenText|' + observed + '|END');
    }
    @IsTest static void datetimeError() {
        String observed = 'accepted';
        try { JSON.deserialize('{"stamp":"2031-04-22 08:30:00.000 UTC"}', Parcel.class); }
        catch (Exception failure) {
            observed = failure.getTypeName() + '|' + failure.getMessage().contains('Cannot deserialize instance of datetime');
        }
        System.assert(false, 'OBSERVED|datetimeError|' + observed + '|END');
    }
    @IsTest static void metadataIdentity() {
        ParcelPolicy__mdt policy = new ParcelPolicy__mdt(MasterLabel='Case');
        String observed = (policy.DeveloperName == null) + '|' + (Type.forName('Schema.' + policy.DeveloperName) == null);
        System.assert(false, 'OBSERVED|metadataIdentity|' + observed + '|END');
    }
}
