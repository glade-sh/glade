@IsTest private class CedarReadOnlyTest {
    @IsTest static void observeReadOnly() {
        for (String phase : new List<String>{'afterInsert', 'afterUpdate', 'beforeDelete', 'afterDelete'}) {
            for (String operation : new List<String>{'put', 'assign', 'putSObject', 'addError', 'listAdd'}) {
                for (Boolean typed : new List<Boolean>{false, true}) {
                    CedarReadOnlyProbe.enabled = false;
                    Contact record = new Contact(LastName='Cedar');
                    if (phase != 'afterInsert') insert record;
                    CedarReadOnlyProbe.phase = phase;
                    CedarReadOnlyProbe.operation = operation;
                    CedarReadOnlyProbe.typed = typed;
                    CedarReadOnlyProbe.enabled = true;
                    String key = phase + '/' + operation + '/' + (typed ? 'typed' : 'generic');
                    Boolean success;
                    String failure;
                    if (phase == 'afterInsert') {
                        Database.SaveResult result = Database.insert(record, false);
                        success = result.isSuccess();
                        if (!success) failure = result.getErrors()[0].getMessage();
                    } else if (phase == 'afterUpdate') {
                        Database.SaveResult result = Database.update(record, false);
                        success = result.isSuccess();
                        if (!success) failure = result.getErrors()[0].getMessage();
                    } else {
                        Database.DeleteResult result = Database.delete(record, false);
                        success = result.isSuccess();
                        if (!success) failure = result.getErrors()[0].getMessage();
                    }
                    if (!CedarReadOnlyProbe.observed.containsKey(key)) {
                        if (success || failure == null) System.assert(false, 'MISSING_FAILURE|' + key);
                        if (failure.contains('System.FinalException: Record is read-only')) CedarReadOnlyProbe.observed.put(key, 'ESCAPED|System.FinalException|Record is read-only');
                        else if (failure.contains('System.FinalException: Collection is read-only')) CedarReadOnlyProbe.observed.put(key, 'ESCAPED|System.FinalException|Collection is read-only');
                        else CedarReadOnlyProbe.observed.put(key, 'UNEXPECTED|' + failure);
                    }
                }
            }
        }
        System.assert(false, 'OBSERVED|' + JSON.serialize(CedarReadOnlyProbe.observed));
    }
    @IsTest static void observeFinalException() {
        String observed = 'miss';
        try { throw new FinalException('probe'); }
        catch (System.FinalException e) { observed = e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
    @IsTest static void observeFinalExceptionGeneric() {
        try { throw new System.FinalException('probe'); }
        catch (Exception e) { System.assert(false, 'CAUGHT|' + e.getTypeName()); }
    }
    @IsTest static void observeFinalExceptionTypeName() {
        Exception e = new FinalException('probe');
        System.assert(false, 'OBSERVED|' + e.getTypeName() + '|' + e.getMessage());
    }
    @IsTest static void observeHandledException() {
        String observed = '';
        try { throw new HandledException('probe'); }
        catch (System.HandledException e) { observed = 'typed|' + e.getTypeName() + '|' + e.getMessage(); }
        try { throw new System.HandledException('probe'); }
        catch (Exception e) { observed += ';generic|' + e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
    @IsTest static void observeUnexpectedException() {
        String observed = '';
        try { throw new UnexpectedException('probe'); }
        catch (System.UnexpectedException e) { observed = 'typed|' + e.getTypeName() + '|' + e.getMessage(); }
        try { throw new System.UnexpectedException('probe'); }
        catch (Exception e) { observed += ';generic|' + e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
    @IsTest static void observeBigObjectException() {
        String observed = '';
        try { throw new BigObjectException('probe'); }
        catch (System.BigObjectException e) { observed = 'typed|' + e.getTypeName() + '|' + e.getMessage(); }
        try { throw new System.BigObjectException('probe'); }
        catch (Exception e) { observed += ';generic|' + e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
    @IsTest static void observeCanvasException() {
        String observed = '';
        try { throw new CanvasException('probe'); }
        catch (System.CanvasException e) { observed = 'typed|' + e.getTypeName() + '|' + e.getMessage(); }
        try { throw new System.CanvasException('probe'); }
        catch (Exception e) { observed += ';generic|' + e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
    @IsTest static void observeFlowException() {
        String observed = '';
        try { throw new FlowException('probe'); }
        catch (System.FlowException e) { observed = 'typed|' + e.getTypeName() + '|' + e.getMessage(); }
        try { throw new System.FlowException('probe'); }
        catch (Exception e) { observed += ';generic|' + e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
    @IsTest static void observeFormulaEvaluationException() {
        String observed = '';
        try { throw new FormulaEvaluationException('probe'); }
        catch (System.FormulaEvaluationException e) { observed = 'typed|' + e.getTypeName() + '|' + e.getMessage(); }
        try { throw new System.FormulaEvaluationException('probe'); }
        catch (Exception e) { observed += ';generic|' + e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
    @IsTest static void observeInvalidReadOnlyUserDmlException() {
        String observed = '';
        try { throw new InvalidReadOnlyUserDmlException('probe'); }
        catch (System.InvalidReadOnlyUserDmlException e) { observed = 'typed|' + e.getTypeName() + '|' + e.getMessage(); }
        try { throw new System.InvalidReadOnlyUserDmlException('probe'); }
        catch (Exception e) { observed += ';generic|' + e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
    @IsTest static void observeLicenseException() {
        String observed = '';
        try { throw new LicenseException('probe'); }
        catch (System.LicenseException e) { observed = 'typed|' + e.getTypeName() + '|' + e.getMessage(); }
        try { throw new System.LicenseException('probe'); }
        catch (Exception e) { observed += ';generic|' + e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
    @IsTest static void observePlatformCacheException() {
        String observed = '';
        try { throw new PlatformCacheException('probe'); }
        catch (System.PlatformCacheException e) { observed = 'typed|' + e.getTypeName() + '|' + e.getMessage(); }
        try { throw new System.PlatformCacheException('probe'); }
        catch (Exception e) { observed += ';generic|' + e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
    @IsTest static void observeEmailTemplateRenderException() {
        String observed = '';
        try { throw new EmailTemplateRenderException('probe'); }
        catch (System.EmailTemplateRenderException e) { observed = 'typed|' + e.getTypeName() + '|' + e.getMessage(); }
        try { throw new System.EmailTemplateRenderException('probe'); }
        catch (Exception e) { observed += ';generic|' + e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
    @IsTest static void observeTransientCursorException() {
        String observed = '';
        try { throw new TransientCursorException('probe'); }
        catch (System.TransientCursorException e) { observed = 'typed|' + e.getTypeName() + '|' + e.getMessage(); }
        try { throw new System.TransientCursorException('probe'); }
        catch (Exception e) { observed += ';generic|' + e.getTypeName() + '|' + e.getMessage(); }
        System.assert(false, 'OBSERVED|' + observed);
    }
}
