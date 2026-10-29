@IsTest private class VisibilityNativeTest { @IsTest static void observe() {
System.assertNotEquals(null, new PrivateParamDefaultException());
System.assertNotEquals(null, new ProtectedParamDefaultException());
System.assertNotEquals(null, new PublicParamDefaultException());
System.assertNotEquals(null, new OrdinaryProtectedZeroDefaultChild());
System.assertNotEquals(null, new OrdinaryProtectedZeroExplicitChild());
System.assertNotEquals(null, new OrdinaryProtectedZeroSuperChild());
System.assertNotEquals(null, new PrivateParamCustomException(1));
System.assertNotEquals(null, new PrivateParamCustomSuperException(1));
System.assertNotEquals(null, new ProtectedParamCustomException(1));
System.assertNotEquals(null, new ProtectedParamCustomSuperException(1));
System.assertNotEquals(null, new PublicParamCustomException(1));
System.assertNotEquals(null, new PublicParamCustomSuperException(1));
System.assertNotEquals(null, new ProtectedParamCustomArgsSuperException(1));
System.assertNotEquals(null, new PublicParamCustomArgsSuperException(1));
} }
