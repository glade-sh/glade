@IsTest private class JsonSetterProbeTest {
  @IsTest static void capture_constructor_deserialize() { System.assert(false, 'NATIVE_ROW|' + JSON.serialize(JsonSetterProbe.observe('constructor','deserialize'))); }
  @IsTest static void capture_constructor_strict() { System.assert(false, 'NATIVE_ROW|' + JSON.serialize(JsonSetterProbe.observe('constructor','strict'))); }
  @IsTest static void capture_constructor_new() { System.assert(false, 'NATIVE_ROW|' + JSON.serialize(JsonSetterProbe.observe('constructor','new'))); }
  @IsTest static void capture_initializer_deserialize() { System.assert(false, 'NATIVE_ROW|' + JSON.serialize(JsonSetterProbe.observe('initializer','deserialize'))); }
  @IsTest static void capture_initializer_strict() { System.assert(false, 'NATIVE_ROW|' + JSON.serialize(JsonSetterProbe.observe('initializer','strict'))); }
  @IsTest static void capture_initializer_new() { System.assert(false, 'NATIVE_ROW|' + JSON.serialize(JsonSetterProbe.observe('initializer','new'))); }
  @IsTest static void capture_inherited_deserialize() { System.assert(false, 'NATIVE_ROW|' + JSON.serialize(JsonSetterProbe.observe('inherited','deserialize'))); }
  @IsTest static void capture_inherited_strict() { System.assert(false, 'NATIVE_ROW|' + JSON.serialize(JsonSetterProbe.observe('inherited','strict'))); }
  @IsTest static void capture_inherited_new() { System.assert(false, 'NATIVE_ROW|' + JSON.serialize(JsonSetterProbe.observe('inherited','new'))); }
}
