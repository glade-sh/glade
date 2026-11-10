public class WireCapture62 {
@AuraEnabled public static WireDirect62 direct() {return new WireDirect62();}
@AuraEnabled public static WireFinite62 finite() {return new WireFinite62();}
@AuraEnabled public static WireRecursive62 recursive() {return new WireRecursive62();}
@AuraEnabled public static String control(){return 'ok';}
@AuraEnabled public static String reject(){throw new AuraHandledException('control reject');}
}
