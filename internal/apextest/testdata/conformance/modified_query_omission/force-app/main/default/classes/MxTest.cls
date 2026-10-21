@IsTest private class MxTest {
    static Id mk(Boolean withPlan) {
        PrbPlan__c p = new PrbPlan__c(Name='p'); insert p;
        PrbLedger__c l = new PrbLedger__c(Name='l', Amount__c=12, Count__c=4, Note__c='n'); if (withPlan) { l.Plan__c = p.Id; } insert l; return l.Id;
    }
    static void emit(String tag, String s) { System.assert(false, 'OBS|' + tag + '|' + s); }

    @IsTest static void a24_sta_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA24.run(mk(false), 'none', false));
        parts.add('add=' + MxA24.run(mk(false), 'add', false));
        parts.add('sub=' + MxA24.run(mk(false), 'sub', false));
        parts.add('addx=' + MxA24.run(mk(false), 'addx', false));
        parts.add('set=' + MxA24.run(mk(false), 'set', false));
        parts.add('txt=' + MxA24.run(mk(false), 'txt', false));
        parts.add('inc=' + MxA24.run(mk(false), 'inc', false));
        parts.add('put=' + MxA24.run(mk(false), 'put', false));
        parts.add('cnt=' + MxA24.run(mk(false), 'cnt', false));
        parts.add('amtread=' + MxA24.run(mk(false), 'amtread', false));
        emit('A24', String.join(parts, ' ')); }
    @IsTest static void a24_sta_plan() { List<String> parts = new List<String>();
        parts.add('none=' + MxA24.run(mk(true), 'none', false));
        parts.add('add=' + MxA24.run(mk(true), 'add', false));
        parts.add('sub=' + MxA24.run(mk(true), 'sub', false));
        parts.add('addx=' + MxA24.run(mk(true), 'addx', false));
        parts.add('set=' + MxA24.run(mk(true), 'set', false));
        parts.add('txt=' + MxA24.run(mk(true), 'txt', false));
        parts.add('inc=' + MxA24.run(mk(true), 'inc', false));
        parts.add('put=' + MxA24.run(mk(true), 'put', false));
        parts.add('cnt=' + MxA24.run(mk(true), 'cnt', false));
        parts.add('amtread=' + MxA24.run(mk(true), 'amtread', false));
        emit('A24', String.join(parts, ' ')); }
    @IsTest static void a24_dyn_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA24.run(mk(false), 'none', true));
        parts.add('add=' + MxA24.run(mk(false), 'add', true));
        parts.add('sub=' + MxA24.run(mk(false), 'sub', true));
        parts.add('addx=' + MxA24.run(mk(false), 'addx', true));
        parts.add('set=' + MxA24.run(mk(false), 'set', true));
        parts.add('txt=' + MxA24.run(mk(false), 'txt', true));
        parts.add('inc=' + MxA24.run(mk(false), 'inc', true));
        parts.add('put=' + MxA24.run(mk(false), 'put', true));
        parts.add('cnt=' + MxA24.run(mk(false), 'cnt', true));
        parts.add('amtread=' + MxA24.run(mk(false), 'amtread', true));
        emit('A24', String.join(parts, ' ')); }
    @IsTest static void a24_dyn_plan() { List<String> parts = new List<String>();
        parts.add('none=' + MxA24.run(mk(true), 'none', true));
        parts.add('add=' + MxA24.run(mk(true), 'add', true));
        parts.add('sub=' + MxA24.run(mk(true), 'sub', true));
        parts.add('addx=' + MxA24.run(mk(true), 'addx', true));
        parts.add('set=' + MxA24.run(mk(true), 'set', true));
        parts.add('txt=' + MxA24.run(mk(true), 'txt', true));
        parts.add('inc=' + MxA24.run(mk(true), 'inc', true));
        parts.add('put=' + MxA24.run(mk(true), 'put', true));
        parts.add('cnt=' + MxA24.run(mk(true), 'cnt', true));
        parts.add('amtread=' + MxA24.run(mk(true), 'amtread', true));
        emit('A24', String.join(parts, ' ')); }
    @IsTest static void a31_sta_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA31.run(mk(false), 'none', false));
        parts.add('add=' + MxA31.run(mk(false), 'add', false));
        parts.add('sub=' + MxA31.run(mk(false), 'sub', false));
        parts.add('addx=' + MxA31.run(mk(false), 'addx', false));
        parts.add('set=' + MxA31.run(mk(false), 'set', false));
        parts.add('txt=' + MxA31.run(mk(false), 'txt', false));
        parts.add('inc=' + MxA31.run(mk(false), 'inc', false));
        parts.add('put=' + MxA31.run(mk(false), 'put', false));
        parts.add('cnt=' + MxA31.run(mk(false), 'cnt', false));
        parts.add('amtread=' + MxA31.run(mk(false), 'amtread', false));
        emit('A31', String.join(parts, ' ')); }
    @IsTest static void a31_dyn_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA31.run(mk(false), 'none', true));
        parts.add('add=' + MxA31.run(mk(false), 'add', true));
        parts.add('sub=' + MxA31.run(mk(false), 'sub', true));
        parts.add('addx=' + MxA31.run(mk(false), 'addx', true));
        parts.add('set=' + MxA31.run(mk(false), 'set', true));
        parts.add('txt=' + MxA31.run(mk(false), 'txt', true));
        parts.add('inc=' + MxA31.run(mk(false), 'inc', true));
        parts.add('put=' + MxA31.run(mk(false), 'put', true));
        parts.add('cnt=' + MxA31.run(mk(false), 'cnt', true));
        parts.add('amtread=' + MxA31.run(mk(false), 'amtread', true));
        emit('A31', String.join(parts, ' ')); }
    @IsTest static void a39_sta_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA39.run(mk(false), 'none', false));
        parts.add('add=' + MxA39.run(mk(false), 'add', false));
        parts.add('sub=' + MxA39.run(mk(false), 'sub', false));
        parts.add('addx=' + MxA39.run(mk(false), 'addx', false));
        parts.add('set=' + MxA39.run(mk(false), 'set', false));
        parts.add('txt=' + MxA39.run(mk(false), 'txt', false));
        parts.add('inc=' + MxA39.run(mk(false), 'inc', false));
        parts.add('put=' + MxA39.run(mk(false), 'put', false));
        parts.add('cnt=' + MxA39.run(mk(false), 'cnt', false));
        parts.add('amtread=' + MxA39.run(mk(false), 'amtread', false));
        emit('A39', String.join(parts, ' ')); }
    @IsTest static void a39_sta_plan() { List<String> parts = new List<String>();
        parts.add('none=' + MxA39.run(mk(true), 'none', false));
        parts.add('add=' + MxA39.run(mk(true), 'add', false));
        parts.add('sub=' + MxA39.run(mk(true), 'sub', false));
        parts.add('addx=' + MxA39.run(mk(true), 'addx', false));
        parts.add('set=' + MxA39.run(mk(true), 'set', false));
        parts.add('txt=' + MxA39.run(mk(true), 'txt', false));
        parts.add('inc=' + MxA39.run(mk(true), 'inc', false));
        parts.add('put=' + MxA39.run(mk(true), 'put', false));
        parts.add('cnt=' + MxA39.run(mk(true), 'cnt', false));
        parts.add('amtread=' + MxA39.run(mk(true), 'amtread', false));
        emit('A39', String.join(parts, ' ')); }
    @IsTest static void a39_dyn_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA39.run(mk(false), 'none', true));
        parts.add('add=' + MxA39.run(mk(false), 'add', true));
        parts.add('sub=' + MxA39.run(mk(false), 'sub', true));
        parts.add('addx=' + MxA39.run(mk(false), 'addx', true));
        parts.add('set=' + MxA39.run(mk(false), 'set', true));
        parts.add('txt=' + MxA39.run(mk(false), 'txt', true));
        parts.add('inc=' + MxA39.run(mk(false), 'inc', true));
        parts.add('put=' + MxA39.run(mk(false), 'put', true));
        parts.add('cnt=' + MxA39.run(mk(false), 'cnt', true));
        parts.add('amtread=' + MxA39.run(mk(false), 'amtread', true));
        emit('A39', String.join(parts, ' ')); }
    @IsTest static void a39_dyn_plan() { List<String> parts = new List<String>();
        parts.add('none=' + MxA39.run(mk(true), 'none', true));
        parts.add('add=' + MxA39.run(mk(true), 'add', true));
        parts.add('sub=' + MxA39.run(mk(true), 'sub', true));
        parts.add('addx=' + MxA39.run(mk(true), 'addx', true));
        parts.add('set=' + MxA39.run(mk(true), 'set', true));
        parts.add('txt=' + MxA39.run(mk(true), 'txt', true));
        parts.add('inc=' + MxA39.run(mk(true), 'inc', true));
        parts.add('put=' + MxA39.run(mk(true), 'put', true));
        parts.add('cnt=' + MxA39.run(mk(true), 'cnt', true));
        parts.add('amtread=' + MxA39.run(mk(true), 'amtread', true));
        emit('A39', String.join(parts, ' ')); }
    @IsTest static void a40_sta_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA40.run(mk(false), 'none', false));
        parts.add('add=' + MxA40.run(mk(false), 'add', false));
        parts.add('sub=' + MxA40.run(mk(false), 'sub', false));
        parts.add('addx=' + MxA40.run(mk(false), 'addx', false));
        parts.add('set=' + MxA40.run(mk(false), 'set', false));
        parts.add('txt=' + MxA40.run(mk(false), 'txt', false));
        parts.add('inc=' + MxA40.run(mk(false), 'inc', false));
        parts.add('put=' + MxA40.run(mk(false), 'put', false));
        parts.add('cnt=' + MxA40.run(mk(false), 'cnt', false));
        parts.add('amtread=' + MxA40.run(mk(false), 'amtread', false));
        emit('A40', String.join(parts, ' ')); }
    @IsTest static void a40_sta_plan() { List<String> parts = new List<String>();
        parts.add('none=' + MxA40.run(mk(true), 'none', false));
        parts.add('add=' + MxA40.run(mk(true), 'add', false));
        parts.add('sub=' + MxA40.run(mk(true), 'sub', false));
        parts.add('addx=' + MxA40.run(mk(true), 'addx', false));
        parts.add('set=' + MxA40.run(mk(true), 'set', false));
        parts.add('txt=' + MxA40.run(mk(true), 'txt', false));
        parts.add('inc=' + MxA40.run(mk(true), 'inc', false));
        parts.add('put=' + MxA40.run(mk(true), 'put', false));
        parts.add('cnt=' + MxA40.run(mk(true), 'cnt', false));
        parts.add('amtread=' + MxA40.run(mk(true), 'amtread', false));
        emit('A40', String.join(parts, ' ')); }
    @IsTest static void a40_dyn_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA40.run(mk(false), 'none', true));
        parts.add('add=' + MxA40.run(mk(false), 'add', true));
        parts.add('sub=' + MxA40.run(mk(false), 'sub', true));
        parts.add('addx=' + MxA40.run(mk(false), 'addx', true));
        parts.add('set=' + MxA40.run(mk(false), 'set', true));
        parts.add('txt=' + MxA40.run(mk(false), 'txt', true));
        parts.add('inc=' + MxA40.run(mk(false), 'inc', true));
        parts.add('put=' + MxA40.run(mk(false), 'put', true));
        parts.add('cnt=' + MxA40.run(mk(false), 'cnt', true));
        parts.add('amtread=' + MxA40.run(mk(false), 'amtread', true));
        emit('A40', String.join(parts, ' ')); }
    @IsTest static void a40_dyn_plan() { List<String> parts = new List<String>();
        parts.add('none=' + MxA40.run(mk(true), 'none', true));
        parts.add('add=' + MxA40.run(mk(true), 'add', true));
        parts.add('sub=' + MxA40.run(mk(true), 'sub', true));
        parts.add('addx=' + MxA40.run(mk(true), 'addx', true));
        parts.add('set=' + MxA40.run(mk(true), 'set', true));
        parts.add('txt=' + MxA40.run(mk(true), 'txt', true));
        parts.add('inc=' + MxA40.run(mk(true), 'inc', true));
        parts.add('put=' + MxA40.run(mk(true), 'put', true));
        parts.add('cnt=' + MxA40.run(mk(true), 'cnt', true));
        parts.add('amtread=' + MxA40.run(mk(true), 'amtread', true));
        emit('A40', String.join(parts, ' ')); }
    @IsTest static void a45_sta_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA45.run(mk(false), 'none', false));
        parts.add('add=' + MxA45.run(mk(false), 'add', false));
        parts.add('sub=' + MxA45.run(mk(false), 'sub', false));
        parts.add('addx=' + MxA45.run(mk(false), 'addx', false));
        parts.add('set=' + MxA45.run(mk(false), 'set', false));
        parts.add('txt=' + MxA45.run(mk(false), 'txt', false));
        parts.add('inc=' + MxA45.run(mk(false), 'inc', false));
        parts.add('put=' + MxA45.run(mk(false), 'put', false));
        parts.add('cnt=' + MxA45.run(mk(false), 'cnt', false));
        parts.add('amtread=' + MxA45.run(mk(false), 'amtread', false));
        emit('A45', String.join(parts, ' ')); }
    @IsTest static void a45_dyn_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA45.run(mk(false), 'none', true));
        parts.add('add=' + MxA45.run(mk(false), 'add', true));
        parts.add('sub=' + MxA45.run(mk(false), 'sub', true));
        parts.add('addx=' + MxA45.run(mk(false), 'addx', true));
        parts.add('set=' + MxA45.run(mk(false), 'set', true));
        parts.add('txt=' + MxA45.run(mk(false), 'txt', true));
        parts.add('inc=' + MxA45.run(mk(false), 'inc', true));
        parts.add('put=' + MxA45.run(mk(false), 'put', true));
        parts.add('cnt=' + MxA45.run(mk(false), 'cnt', true));
        parts.add('amtread=' + MxA45.run(mk(false), 'amtread', true));
        emit('A45', String.join(parts, ' ')); }
    @IsTest static void a50_sta_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA50.run(mk(false), 'none', false));
        parts.add('add=' + MxA50.run(mk(false), 'add', false));
        parts.add('sub=' + MxA50.run(mk(false), 'sub', false));
        parts.add('addx=' + MxA50.run(mk(false), 'addx', false));
        parts.add('set=' + MxA50.run(mk(false), 'set', false));
        parts.add('txt=' + MxA50.run(mk(false), 'txt', false));
        parts.add('inc=' + MxA50.run(mk(false), 'inc', false));
        parts.add('put=' + MxA50.run(mk(false), 'put', false));
        parts.add('cnt=' + MxA50.run(mk(false), 'cnt', false));
        parts.add('amtread=' + MxA50.run(mk(false), 'amtread', false));
        emit('A50', String.join(parts, ' ')); }
    @IsTest static void a50_dyn_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA50.run(mk(false), 'none', true));
        parts.add('add=' + MxA50.run(mk(false), 'add', true));
        parts.add('sub=' + MxA50.run(mk(false), 'sub', true));
        parts.add('addx=' + MxA50.run(mk(false), 'addx', true));
        parts.add('set=' + MxA50.run(mk(false), 'set', true));
        parts.add('txt=' + MxA50.run(mk(false), 'txt', true));
        parts.add('inc=' + MxA50.run(mk(false), 'inc', true));
        parts.add('put=' + MxA50.run(mk(false), 'put', true));
        parts.add('cnt=' + MxA50.run(mk(false), 'cnt', true));
        parts.add('amtread=' + MxA50.run(mk(false), 'amtread', true));
        emit('A50', String.join(parts, ' ')); }
    @IsTest static void a62_sta_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA62.run(mk(false), 'none', false));
        parts.add('add=' + MxA62.run(mk(false), 'add', false));
        parts.add('sub=' + MxA62.run(mk(false), 'sub', false));
        parts.add('addx=' + MxA62.run(mk(false), 'addx', false));
        parts.add('set=' + MxA62.run(mk(false), 'set', false));
        parts.add('txt=' + MxA62.run(mk(false), 'txt', false));
        parts.add('inc=' + MxA62.run(mk(false), 'inc', false));
        parts.add('put=' + MxA62.run(mk(false), 'put', false));
        parts.add('cnt=' + MxA62.run(mk(false), 'cnt', false));
        parts.add('amtread=' + MxA62.run(mk(false), 'amtread', false));
        emit('A62', String.join(parts, ' ')); }
    @IsTest static void a62_sta_plan() { List<String> parts = new List<String>();
        parts.add('none=' + MxA62.run(mk(true), 'none', false));
        parts.add('add=' + MxA62.run(mk(true), 'add', false));
        parts.add('sub=' + MxA62.run(mk(true), 'sub', false));
        parts.add('addx=' + MxA62.run(mk(true), 'addx', false));
        parts.add('set=' + MxA62.run(mk(true), 'set', false));
        parts.add('txt=' + MxA62.run(mk(true), 'txt', false));
        parts.add('inc=' + MxA62.run(mk(true), 'inc', false));
        parts.add('put=' + MxA62.run(mk(true), 'put', false));
        parts.add('cnt=' + MxA62.run(mk(true), 'cnt', false));
        parts.add('amtread=' + MxA62.run(mk(true), 'amtread', false));
        emit('A62', String.join(parts, ' ')); }
    @IsTest static void a62_dyn_null() { List<String> parts = new List<String>();
        parts.add('none=' + MxA62.run(mk(false), 'none', true));
        parts.add('add=' + MxA62.run(mk(false), 'add', true));
        parts.add('sub=' + MxA62.run(mk(false), 'sub', true));
        parts.add('addx=' + MxA62.run(mk(false), 'addx', true));
        parts.add('set=' + MxA62.run(mk(false), 'set', true));
        parts.add('txt=' + MxA62.run(mk(false), 'txt', true));
        parts.add('inc=' + MxA62.run(mk(false), 'inc', true));
        parts.add('put=' + MxA62.run(mk(false), 'put', true));
        parts.add('cnt=' + MxA62.run(mk(false), 'cnt', true));
        parts.add('amtread=' + MxA62.run(mk(false), 'amtread', true));
        emit('A62', String.join(parts, ' ')); }
    @IsTest static void a62_dyn_plan() { List<String> parts = new List<String>();
        parts.add('none=' + MxA62.run(mk(true), 'none', true));
        parts.add('add=' + MxA62.run(mk(true), 'add', true));
        parts.add('sub=' + MxA62.run(mk(true), 'sub', true));
        parts.add('addx=' + MxA62.run(mk(true), 'addx', true));
        parts.add('set=' + MxA62.run(mk(true), 'set', true));
        parts.add('txt=' + MxA62.run(mk(true), 'txt', true));
        parts.add('inc=' + MxA62.run(mk(true), 'inc', true));
        parts.add('put=' + MxA62.run(mk(true), 'put', true));
        parts.add('cnt=' + MxA62.run(mk(true), 'cnt', true));
        parts.add('amtread=' + MxA62.run(mk(true), 'amtread', true));
        emit('A62', String.join(parts, ' ')); }
    @IsTest static void b_q24_w24() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW24.apply(r, 'none'); parts.add('W24R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW24.apply(r, 'add'); parts.add('W24R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW24.apply(r, 'none'); parts.add('W24R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW24.apply(r, 'add'); parts.add('W24R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW24.apply(r, 'none'); parts.add('W24R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW24.apply(r, 'add'); parts.add('W24R62add=' + MxR62.rd(r)); }
        emit('BQ24W24', String.join(parts, ' ')); }
    @IsTest static void b_q24_w39() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW39.apply(r, 'none'); parts.add('W39R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW39.apply(r, 'add'); parts.add('W39R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW39.apply(r, 'none'); parts.add('W39R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW39.apply(r, 'add'); parts.add('W39R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW39.apply(r, 'none'); parts.add('W39R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW39.apply(r, 'add'); parts.add('W39R62add=' + MxR62.rd(r)); }
        emit('BQ24W39', String.join(parts, ' ')); }
    @IsTest static void b_q24_w40() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW40.apply(r, 'none'); parts.add('W40R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW40.apply(r, 'add'); parts.add('W40R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW40.apply(r, 'none'); parts.add('W40R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW40.apply(r, 'add'); parts.add('W40R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW40.apply(r, 'none'); parts.add('W40R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW40.apply(r, 'add'); parts.add('W40R62add=' + MxR62.rd(r)); }
        emit('BQ24W40', String.join(parts, ' ')); }
    @IsTest static void b_q24_w62() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW62.apply(r, 'none'); parts.add('W62R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW62.apply(r, 'add'); parts.add('W62R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW62.apply(r, 'none'); parts.add('W62R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW62.apply(r, 'add'); parts.add('W62R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW62.apply(r, 'none'); parts.add('W62R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ24.fetch(mk(false)); MxW62.apply(r, 'add'); parts.add('W62R62add=' + MxR62.rd(r)); }
        emit('BQ24W62', String.join(parts, ' ')); }
    @IsTest static void b_q40_w24() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW24.apply(r, 'none'); parts.add('W24R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW24.apply(r, 'add'); parts.add('W24R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW24.apply(r, 'none'); parts.add('W24R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW24.apply(r, 'add'); parts.add('W24R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW24.apply(r, 'none'); parts.add('W24R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW24.apply(r, 'add'); parts.add('W24R62add=' + MxR62.rd(r)); }
        emit('BQ40W24', String.join(parts, ' ')); }
    @IsTest static void b_q40_w39() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW39.apply(r, 'none'); parts.add('W39R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW39.apply(r, 'add'); parts.add('W39R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW39.apply(r, 'none'); parts.add('W39R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW39.apply(r, 'add'); parts.add('W39R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW39.apply(r, 'none'); parts.add('W39R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW39.apply(r, 'add'); parts.add('W39R62add=' + MxR62.rd(r)); }
        emit('BQ40W39', String.join(parts, ' ')); }
    @IsTest static void b_q40_w40() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW40.apply(r, 'none'); parts.add('W40R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW40.apply(r, 'add'); parts.add('W40R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW40.apply(r, 'none'); parts.add('W40R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW40.apply(r, 'add'); parts.add('W40R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW40.apply(r, 'none'); parts.add('W40R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW40.apply(r, 'add'); parts.add('W40R62add=' + MxR62.rd(r)); }
        emit('BQ40W40', String.join(parts, ' ')); }
    @IsTest static void b_q40_w62() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW62.apply(r, 'none'); parts.add('W62R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW62.apply(r, 'add'); parts.add('W62R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW62.apply(r, 'none'); parts.add('W62R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW62.apply(r, 'add'); parts.add('W62R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW62.apply(r, 'none'); parts.add('W62R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ40.fetch(mk(false)); MxW62.apply(r, 'add'); parts.add('W62R62add=' + MxR62.rd(r)); }
        emit('BQ40W62', String.join(parts, ' ')); }
    @IsTest static void b_q62_w24() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW24.apply(r, 'none'); parts.add('W24R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW24.apply(r, 'add'); parts.add('W24R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW24.apply(r, 'none'); parts.add('W24R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW24.apply(r, 'add'); parts.add('W24R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW24.apply(r, 'none'); parts.add('W24R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW24.apply(r, 'add'); parts.add('W24R62add=' + MxR62.rd(r)); }
        emit('BQ62W24', String.join(parts, ' ')); }
    @IsTest static void b_q62_w39() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW39.apply(r, 'none'); parts.add('W39R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW39.apply(r, 'add'); parts.add('W39R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW39.apply(r, 'none'); parts.add('W39R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW39.apply(r, 'add'); parts.add('W39R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW39.apply(r, 'none'); parts.add('W39R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW39.apply(r, 'add'); parts.add('W39R62add=' + MxR62.rd(r)); }
        emit('BQ62W39', String.join(parts, ' ')); }
    @IsTest static void b_q62_w40() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW40.apply(r, 'none'); parts.add('W40R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW40.apply(r, 'add'); parts.add('W40R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW40.apply(r, 'none'); parts.add('W40R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW40.apply(r, 'add'); parts.add('W40R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW40.apply(r, 'none'); parts.add('W40R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW40.apply(r, 'add'); parts.add('W40R62add=' + MxR62.rd(r)); }
        emit('BQ62W40', String.join(parts, ' ')); }
    @IsTest static void b_q62_w62() { List<String> parts = new List<String>();
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW62.apply(r, 'none'); parts.add('W62R24none=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW62.apply(r, 'add'); parts.add('W62R24add=' + MxR24.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW62.apply(r, 'none'); parts.add('W62R40none=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW62.apply(r, 'add'); parts.add('W62R40add=' + MxR40.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW62.apply(r, 'none'); parts.add('W62R62none=' + MxR62.rd(r)); }
        { PrbLedger__c r = MxQ62.fetch(mk(false)); MxW62.apply(r, 'add'); parts.add('W62R62add=' + MxR62.rd(r)); }
        emit('BQ62W62', String.join(parts, ' ')); }
}