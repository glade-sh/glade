public class C155NITriggerView {
    public static Boolean enabled=false;
    public static String selectedField='Qty__c';
    public static Map<String,Object> views=new Map<String,Object>();
    private static Map<String,Object> snapshot(Object v) {
        Map<String,Object> o=new Map<String,Object>();
        o.put('Integer',v instanceof Integer);o.put('Long',v instanceof Long);o.put('Decimal',v instanceof Decimal);o.put('isNull',v==null);o.put('text',String.valueOf(v));o.put('json',JSON.serialize(v));try{o.put('castInteger',String.valueOf((Integer)v));}catch(Exception e){o.put('castInteger',new List<String>{e.getTypeName(),e.getMessage()});}try{o.put('castLong',String.valueOf((Long)v));}catch(Exception e){o.put('castLong',new List<String>{e.getTypeName(),e.getMessage()});}try{o.put('castDecimal',String.valueOf((Decimal)v));}catch(Exception e){o.put('castDecimal',new List<String>{e.getTypeName(),e.getMessage()});}
        return o;
    }
    public static void observe(C155Number__c w,String phase) {
        if(!enabled){return;}
        views.put(phase+'.get',snapshot(w.get(selectedField)));
        Object typed;
        if(selectedField=='Qty__c'){typed=w.Qty__c;}
        else if(selectedField=='Amt__c'){typed=w.Amt__c;}
        else{typed=w.Cur__c;}
        views.put(phase+'.typed',snapshot(typed));
    }
}
