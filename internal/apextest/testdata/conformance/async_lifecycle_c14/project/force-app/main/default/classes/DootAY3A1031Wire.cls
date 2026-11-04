public without sharing class DootAY3A1031Wire {
 public static String bounded(Map<String,Object> d){String s=JSON.serialize(d);if(s.length()>620)return JSON.serialize(new Map<String,Object>{'status'=>'inconclusive_overlength','chars'=>s.length()});return s;}
 public static Id fixture(){ DootAY3A1031__c x=new DootAY3A1031__c(Name='owned',Data__c='{"cb":"pending"}');insert as system x;return x.Id;}
 public static void put(Id owned,Map<String,Object> d){DootAY3A1031__c x=new DootAY3A1031__c(Id=owned,Data__c=JSON.serialize(d));update as system x;}
 public static Object read(Id owned){List<DootAY3A1031__c> xs=[SELECT Data__c FROM DootAY3A1031__c WHERE Id=:owned WITH SYSTEM_MODE];if(xs.size()!=1)return new Map<String,Object>{'status'=>'inconclusive_owned_missing'};return JSON.deserializeUntyped(xs[0].Data__c);}
 public static void error(Map<String,Object> d,Exception e,String stage){d.put('error_'+stage,new Map<String,Object>{'type'=>e.getTypeName(),'message'=>e.getMessage(),'line'=>e.getLineNumber()});}
 public static void emit(String id,Map<String,Object> d){System.assert(false,'CAPTURE|'+id+'|'+bounded(d));}
}
