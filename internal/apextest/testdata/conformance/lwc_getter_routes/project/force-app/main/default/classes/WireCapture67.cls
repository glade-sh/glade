public class WireCapture67 {
@AuraEnabled public static WireDirect67 direct() {return new WireDirect67();}
@AuraEnabled public static WireFinite67 finite() {return new WireFinite67();}
@AuraEnabled public static WireRecursive67 recursive() {return new WireRecursive67();}
@AuraEnabled public static String control(){return 'ok';}
@AuraEnabled public static String reject(){throw new AuraHandledException('control reject');}
}
