public class WireRecursive62 { @AuraEnabled public Integer visits=0; @AuraEnabled public String value { get {return helper();} set; } public String helper(){return value;} }
