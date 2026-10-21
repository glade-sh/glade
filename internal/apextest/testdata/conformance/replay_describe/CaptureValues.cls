public class C168CaptureValues {
    public Object normalize(Object raw, String key) {
        if (raw == null) {
            if (key == 'EventUuid' || key == 'uuid' || key == 'id' || key == 'CreatedById') return new Map<String,Object>{'isNull'=>true};
            return null;
        }
        if (raw instanceof Map<String,Object>) {
            Map<String,Object> source=(Map<String,Object>)raw;
            Map<String,Object> result=new Map<String,Object>();
            String status=source.containsKey('status') ? String.valueOf(source.get('status')) : null;
            for (String child : source.keySet()) {
                Object item=source.get(child);
                if ((child == 'message' || child == 'exceptionMessage') && item != null) {
                    String text=String.valueOf(item);
                    if (status == 'OPERATION_ENQUEUED') result.put(child,new Map<String,Object>{'length'=>text.length(),'uuidShaped'=>Pattern.matches('(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}',text)});
                    else result.put(child,message(text));
                } else result.put(child,normalize(item,child));
            }
            return result;
        }
        if (raw instanceof List<Object>) {
            List<Object> result=new List<Object>();
            for (Object item : (List<Object>)raw) result.add(normalize(item,key));
            return result;
        }
        if (raw instanceof String) {
            String text=(String)raw;
            if (key == 'EventUuid' || key == 'uuid' || Pattern.matches('(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}',text)) return new Map<String,Object>{'isNull'=>false,'length'=>text.length(),'uuidShaped'=>Pattern.matches('(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}',text)};
            if (Pattern.matches('[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?',text)) {
                if (text.startsWith('005')) return new Map<String,Object>{'isRunningUser'=>text == String.valueOf(UserInfo.getUserId()),'idPrefix'=>text.left(3),'idLength'=>text.length()};
                return new Map<String,Object>{'idPrefix'=>text.left(3),'idLength'=>text.length()};
            }
            return fixtureNames(text);
        }
        return raw;
    }
    public String fixtureNames(String text) {
        return text.replaceAll('C168A(?:59|6[2-8])[a-f0-9]{8}Event__e','SyntheticEvent__e').replaceAll('C168A(?:59|6[2-8])[a-f0-9]{8}Record__c','SyntheticRecord__c');
    }
    public Object message(String original) {
        String text=fixtureNames(original).replaceAll('(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}','[UUID]').replaceAll('(?<![A-Za-z0-9])[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?(?![A-Za-z0-9])','[ID]');
        return new Map<String,Object>{'text'=>text.left(200),'length'=>original.length(),'truncated'=>text.length()>200};
    }
public void emit(Object v,String cid){Object captured=normalize(JSON.deserializeUntyped(JSON.serialize(v)),'');System.assert(false,'E15_RESULT|'+cid+'|'+JSON.serialize(new Map<String,Object>{'value'=>captured}));}}