@IsTest private class MyTest {
    static Id mk() { PrbLedger__c l = new PrbLedger__c(Name='l', Amount__c=12, Count__c=4, Note__c='n'); insert l; return l.Id; }
    static void emit(String tag, String s) { System.assert(false, 'OBS|' + tag + '|' + s); }
    static PrbLedger__c q(Id i) { return [SELECT Id,Name,Amount__c,Count__c FROM PrbLedger__c WHERE Id = :i]; }
    @IsTest static void c_R24() { List<String> parts = new List<String>();
        parts.add('none=' + MyR24.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyR24.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyR24.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyR24.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyR24.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyR24.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyR24.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyR24.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyR24.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyR24.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('R24', String.join(parts, ' ')); }
    @IsTest static void c_R40() { List<String> parts = new List<String>();
        parts.add('none=' + MyR40.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyR40.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyR40.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyR40.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyR40.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyR40.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyR40.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyR40.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyR40.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyR40.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('R40', String.join(parts, ' ')); }
    @IsTest static void c_R41() { List<String> parts = new List<String>();
        parts.add('none=' + MyR41.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyR41.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyR41.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyR41.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyR41.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyR41.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyR41.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyR41.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyR41.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyR41.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('R41', String.join(parts, ' ')); }
    @IsTest static void c_R42() { List<String> parts = new List<String>();
        parts.add('none=' + MyR42.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyR42.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyR42.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyR42.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyR42.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyR42.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyR42.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyR42.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyR42.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyR42.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('R42', String.join(parts, ' ')); }
    @IsTest static void c_R43() { List<String> parts = new List<String>();
        parts.add('none=' + MyR43.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyR43.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyR43.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyR43.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyR43.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyR43.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyR43.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyR43.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyR43.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyR43.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('R43', String.join(parts, ' ')); }
    @IsTest static void c_R44() { List<String> parts = new List<String>();
        parts.add('none=' + MyR44.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyR44.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyR44.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyR44.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyR44.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyR44.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyR44.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyR44.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyR44.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyR44.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('R44', String.join(parts, ' ')); }
    @IsTest static void c_R45() { List<String> parts = new List<String>();
        parts.add('none=' + MyR45.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyR45.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyR45.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyR45.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyR45.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyR45.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyR45.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyR45.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyR45.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyR45.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('R45', String.join(parts, ' ')); }
    @IsTest static void c_R62() { List<String> parts = new List<String>();
        parts.add('none=' + MyR62.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyR62.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyR62.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyR62.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyR62.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyR62.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyR62.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyR62.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyR62.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyR62.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('R62', String.join(parts, ' ')); }
    @IsTest static void c_N40() { List<String> parts = new List<String>();
        parts.add('none=' + MyN40.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyN40.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyN40.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyN40.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyN40.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyN40.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyN40.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyN40.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyN40.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyN40.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('N40', String.join(parts, ' ')); }
    @IsTest static void c_I40() { List<String> parts = new List<String>();
        parts.add('none=' + MyI40.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyI40.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyI40.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyI40.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyI40.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyI40.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyI40.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyI40.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyI40.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyI40.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('I40', String.join(parts, ' ')); }
    @IsTest static void c_P40() { List<String> parts = new List<String>();
        parts.add('none=' + MyP40.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyP40.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyP40.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyP40.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyP40.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyP40.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyP40.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyP40.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyP40.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyP40.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('P40', String.join(parts, ' ')); }
    @IsTest static void c_P24() { List<String> parts = new List<String>();
        parts.add('none=' + MyP24.rd(MyW.apply(q(mk()), 'none')));
        parts.add('add=' + MyP24.rd(MyW.apply(q(mk()), 'add')));
        parts.add('same=' + MyP24.rd(MyW.apply(q(mk()), 'same')));
        parts.add('setun=' + MyP24.rd(MyW.apply(q(mk()), 'setun')));
        parts.add('putun=' + MyP24.rd(MyW.apply(q(mk()), 'putun')));
        parts.add('dml=' + MyP24.rd(MyW.apply(q(mk()), 'dml')));
        parts.add('dmlonly=' + MyP24.rd(MyW.apply(q(mk()), 'dmlonly')));
        parts.add('clone=' + MyP24.rd(MyW.apply(q(mk()), 'clone')));
        parts.add('cloneclean=' + MyP24.rd(MyW.apply(q(mk()), 'cloneclean')));
        parts.add('cloneAfter=' + MyP24.rd(MyW.apply(q(mk()), 'cloneAfter')));
        emit('P24', String.join(parts, ' ')); }
}