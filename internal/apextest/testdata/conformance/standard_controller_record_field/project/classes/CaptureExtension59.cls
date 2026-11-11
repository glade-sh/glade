public class CaptureExtension59 { public Id recordId {get;set;} public CaptureExtension59(ApexPages.StandardController c) { recordId=(c.getRecord()).Id; } }
