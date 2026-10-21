public class HandledMessageProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> observed = new Map<String,Object>();
        if (Test.isRunningTest()) {
            try { throw new AuraHandledException('Owned detail'); }
            catch (AuraHandledException error) { observed.put('thrown',error.getMessage()); }
            try {
                try { throw new AuraHandledException('Owned inner detail'); }
                catch (AuraHandledException error) { throw new AuraHandledException('Owned outer detail ' + error.getMessage()); }
            } catch (AuraHandledException error) { observed.put('wrapped',error.getMessage()); }
            try {
                AuraHandledException error = new AuraHandledException('Owned detail');
                error.setMessage('Owned replacement');
                throw error;
            } catch (AuraHandledException error) { observed.put('explicit',error.getMessage()); }
        } else observed.put('context','anonymous');
        Map<String,List<String>> responses = new Map<String,List<String>>{'owned_first'=>new List<String>{'result'}};
        List<String> fallback = new List<String>();
        observed.put('different_key', responses.containsKey('owned_second') ? responses.get('owned_second') : fallback);
        return observed;
    }
}
