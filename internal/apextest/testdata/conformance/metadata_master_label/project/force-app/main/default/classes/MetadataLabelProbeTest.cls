@IsTest private class MetadataLabelProbeTest {
@IsTest static void staticLabel() { System.assert(false, 'OBSERVED|staticLabel|' + JSON.serialize(MetadataLabelProbe.staticLabel())); }
@IsTest static void dynamicLabel() { System.assert(false, 'OBSERVED|dynamicLabel|' + JSON.serialize(MetadataLabelProbe.dynamicLabel())); }
@IsTest static void emptyLabel() { System.assert(false, 'OBSERVED|emptyLabel|' + JSON.serialize(MetadataLabelProbe.emptyLabel())); }
@IsTest static void invalidField() { System.assert(false, 'OBSERVED|invalidField|' + JSON.serialize(MetadataLabelProbe.invalidField())); }
}
