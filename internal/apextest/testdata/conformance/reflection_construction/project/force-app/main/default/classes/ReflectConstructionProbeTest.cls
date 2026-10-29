@IsTest private class ReflectConstructionProbeTest {
    @IsTest static void userInfo() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('UserInfo'))); }
    @IsTest static void qualifiedUserInfo() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('System.UserInfo'))); }
    @IsTest static void systemNamespace() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('System', 'UserInfo'))); }
    @IsTest static void plain() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('ReflectConstructionProbe.Plain'))); }
    @IsTest static void privateConstructor() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('ReflectConstructionProbe.PrivateConstructor'))); }
    @IsTest static void protectedConstructor() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('ReflectConstructionProbe.ProtectedConstructor'))); }
    @IsTest static void argumentConstructor() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('ReflectConstructionProbe.ArgumentConstructor'))); }
    @IsTest static void abstractValue() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('ReflectConstructionProbe.AbstractValue'))); }
    @IsTest static void contract() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('ReflectConstructionProbe.Contract'))); }
    @IsTest static void throwingConstructor() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('ReflectConstructionProbe.ThrowingConstructor'))); }
    @IsTest static void emptyNamespace() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('', 'ReflectConstructionProbe.Plain'))); }
    @IsTest static void nullNamespace() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName(null, 'ReflectConstructionProbe.Plain'))); }
    @IsTest static void missingNamespace() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('missingzone', 'ReflectConstructionProbe.Plain'))); }
    @IsTest static void literalUserInfo() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(UserInfo.class)); }
    @IsTest static void splitNested() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('ReflectConstructionProbe', 'Plain'))); }
    @IsTest static void repeatedOuter() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('ReflectConstructionProbe', 'ReflectConstructionProbe.Plain'))); }
    @IsTest static void qualifiedMissingNamespace() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('missingzone', 'System.UserInfo'))); }
    @IsTest static void emptyQualifiedUserInfo() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('', 'System.UserInfo'))); }
    @IsTest static void nullQualifiedUserInfo() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName(null, 'System.UserInfo'))); }
    @IsTest static void systemQualifiedUserInfo() { System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(Type.forName('System', 'System.UserInfo'))); }
}
