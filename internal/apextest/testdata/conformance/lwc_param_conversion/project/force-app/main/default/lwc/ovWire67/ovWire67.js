import save from '@salesforce/apex/OvSave.save';
import {LightningElement, wire} from 'lwc';
import fw_54_String from '@salesforce/apex/Cv54.wString';
import fw_54_Integer from '@salesforce/apex/Cv54.wInteger';
import fw_54_Long from '@salesforce/apex/Cv54.wLong';
import fw_54_Decimal from '@salesforce/apex/Cv54.wDecimal';
import fw_54_Double from '@salesforce/apex/Cv54.wDouble';
import fw_54_Boolean from '@salesforce/apex/Cv54.wBoolean';
import fw_54_Date from '@salesforce/apex/Cv54.wDate';
import fw_54_Datetime from '@salesforce/apex/Cv54.wDatetime';
import fw_54_Id from '@salesforce/apex/Cv54.wId';
import fw_54_ListString from '@salesforce/apex/Cv54.wListString';
import fw_54_Object from '@salesforce/apex/Cv54.wObject';
import fw_54_Account from '@salesforce/apex/Cv54.wAccount';
import fw_54_SObject from '@salesforce/apex/Cv54.wSObject';
import fw_62_String from '@salesforce/apex/Cv62.wString';
import fw_62_Integer from '@salesforce/apex/Cv62.wInteger';
import fw_62_Long from '@salesforce/apex/Cv62.wLong';
import fw_62_Decimal from '@salesforce/apex/Cv62.wDecimal';
import fw_62_Double from '@salesforce/apex/Cv62.wDouble';
import fw_62_Boolean from '@salesforce/apex/Cv62.wBoolean';
import fw_62_Date from '@salesforce/apex/Cv62.wDate';
import fw_62_Datetime from '@salesforce/apex/Cv62.wDatetime';
import fw_62_Id from '@salesforce/apex/Cv62.wId';
import fw_62_ListString from '@salesforce/apex/Cv62.wListString';
import fw_62_Object from '@salesforce/apex/Cv62.wObject';
import fw_62_Account from '@salesforce/apex/Cv62.wAccount';
import fw_62_SObject from '@salesforce/apex/Cv62.wSObject';
import fw_67_String from '@salesforce/apex/Cv67.wString';
import fw_67_Integer from '@salesforce/apex/Cv67.wInteger';
import fw_67_Long from '@salesforce/apex/Cv67.wLong';
import fw_67_Decimal from '@salesforce/apex/Cv67.wDecimal';
import fw_67_Double from '@salesforce/apex/Cv67.wDouble';
import fw_67_Boolean from '@salesforce/apex/Cv67.wBoolean';
import fw_67_Date from '@salesforce/apex/Cv67.wDate';
import fw_67_Datetime from '@salesforce/apex/Cv67.wDatetime';
import fw_67_Id from '@salesforce/apex/Cv67.wId';
import fw_67_ListString from '@salesforce/apex/Cv67.wListString';
import fw_67_Object from '@salesforce/apex/Cv67.wObject';
import fw_67_Account from '@salesforce/apex/Cv67.wAccount';
import fw_67_SObject from '@salesforce/apex/Cv67.wSObject';
export default class OvWire extends LightningElement { result='pending'; rows=[]; saved=false;
@wire(fw_67_String, {value:'x'}) h1(r) { this.rec('67','String','str_x',r); }
@wire(fw_67_String, {value:42}) h2(r) { this.rec('67','String','num_42',r); }
@wire(fw_67_String, {value:true}) h3(r) { this.rec('67','String','bool_true',r); }
@wire(fw_67_String, {value:4.5}) h4(r) { this.rec('67','String','num_frac',r); }
@wire(fw_67_String, {value:'42'}) h5(r) { this.rec('67','String','str_42',r); }
@wire(fw_67_String, {value:'2024-01-02'}) h6(r) { this.rec('67','String','str_date',r); }
@wire(fw_67_String, {value:['a','b']}) h7(r) { this.rec('67','String','arr_str',r); }
@wire(fw_67_String, {value:{Name:'n'}}) h8(r) { this.rec('67','String','obj_name',r); }
@wire(fw_67_String, {value:'001000000000000AAA'}) h9(r) { this.rec('67','String','str_id18',r); }
@wire(fw_67_String, {value:false}) h10(r) { this.rec('67','String','bool_false',r); }
@wire(fw_67_Integer, {value:'x'}) h11(r) { this.rec('67','Integer','str_x',r); }
@wire(fw_67_Integer, {value:42}) h12(r) { this.rec('67','Integer','num_42',r); }
@wire(fw_67_Integer, {value:true}) h13(r) { this.rec('67','Integer','bool_true',r); }
@wire(fw_67_Integer, {value:4.5}) h14(r) { this.rec('67','Integer','num_frac',r); }
@wire(fw_67_Integer, {value:'42'}) h15(r) { this.rec('67','Integer','str_42',r); }
@wire(fw_67_Integer, {value:'2024-01-02'}) h16(r) { this.rec('67','Integer','str_date',r); }
@wire(fw_67_Integer, {value:['a','b']}) h17(r) { this.rec('67','Integer','arr_str',r); }
@wire(fw_67_Integer, {value:{Name:'n'}}) h18(r) { this.rec('67','Integer','obj_name',r); }
@wire(fw_67_Integer, {value:'001000000000000AAA'}) h19(r) { this.rec('67','Integer','str_id18',r); }
@wire(fw_67_Integer, {value:false}) h20(r) { this.rec('67','Integer','bool_false',r); }
@wire(fw_67_Long, {value:'x'}) h21(r) { this.rec('67','Long','str_x',r); }
@wire(fw_67_Long, {value:42}) h22(r) { this.rec('67','Long','num_42',r); }
@wire(fw_67_Long, {value:true}) h23(r) { this.rec('67','Long','bool_true',r); }
@wire(fw_67_Long, {value:4.5}) h24(r) { this.rec('67','Long','num_frac',r); }
@wire(fw_67_Long, {value:'42'}) h25(r) { this.rec('67','Long','str_42',r); }
@wire(fw_67_Long, {value:'2024-01-02'}) h26(r) { this.rec('67','Long','str_date',r); }
@wire(fw_67_Long, {value:['a','b']}) h27(r) { this.rec('67','Long','arr_str',r); }
@wire(fw_67_Long, {value:{Name:'n'}}) h28(r) { this.rec('67','Long','obj_name',r); }
@wire(fw_67_Long, {value:'001000000000000AAA'}) h29(r) { this.rec('67','Long','str_id18',r); }
@wire(fw_67_Long, {value:false}) h30(r) { this.rec('67','Long','bool_false',r); }
@wire(fw_67_Decimal, {value:'x'}) h31(r) { this.rec('67','Decimal','str_x',r); }
@wire(fw_67_Decimal, {value:42}) h32(r) { this.rec('67','Decimal','num_42',r); }
@wire(fw_67_Decimal, {value:true}) h33(r) { this.rec('67','Decimal','bool_true',r); }
@wire(fw_67_Decimal, {value:4.5}) h34(r) { this.rec('67','Decimal','num_frac',r); }
@wire(fw_67_Decimal, {value:'42'}) h35(r) { this.rec('67','Decimal','str_42',r); }
@wire(fw_67_Decimal, {value:'2024-01-02'}) h36(r) { this.rec('67','Decimal','str_date',r); }
@wire(fw_67_Decimal, {value:['a','b']}) h37(r) { this.rec('67','Decimal','arr_str',r); }
@wire(fw_67_Decimal, {value:{Name:'n'}}) h38(r) { this.rec('67','Decimal','obj_name',r); }
@wire(fw_67_Decimal, {value:'001000000000000AAA'}) h39(r) { this.rec('67','Decimal','str_id18',r); }
@wire(fw_67_Decimal, {value:false}) h40(r) { this.rec('67','Decimal','bool_false',r); }
@wire(fw_67_Double, {value:'x'}) h41(r) { this.rec('67','Double','str_x',r); }
@wire(fw_67_Double, {value:42}) h42(r) { this.rec('67','Double','num_42',r); }
@wire(fw_67_Double, {value:true}) h43(r) { this.rec('67','Double','bool_true',r); }
@wire(fw_67_Double, {value:4.5}) h44(r) { this.rec('67','Double','num_frac',r); }
@wire(fw_67_Double, {value:'42'}) h45(r) { this.rec('67','Double','str_42',r); }
@wire(fw_67_Double, {value:'2024-01-02'}) h46(r) { this.rec('67','Double','str_date',r); }
@wire(fw_67_Double, {value:['a','b']}) h47(r) { this.rec('67','Double','arr_str',r); }
@wire(fw_67_Double, {value:{Name:'n'}}) h48(r) { this.rec('67','Double','obj_name',r); }
@wire(fw_67_Double, {value:'001000000000000AAA'}) h49(r) { this.rec('67','Double','str_id18',r); }
@wire(fw_67_Double, {value:false}) h50(r) { this.rec('67','Double','bool_false',r); }
@wire(fw_67_Boolean, {value:'x'}) h51(r) { this.rec('67','Boolean','str_x',r); }
@wire(fw_67_Boolean, {value:42}) h52(r) { this.rec('67','Boolean','num_42',r); }
@wire(fw_67_Boolean, {value:true}) h53(r) { this.rec('67','Boolean','bool_true',r); }
@wire(fw_67_Boolean, {value:4.5}) h54(r) { this.rec('67','Boolean','num_frac',r); }
@wire(fw_67_Boolean, {value:'42'}) h55(r) { this.rec('67','Boolean','str_42',r); }
@wire(fw_67_Boolean, {value:'2024-01-02'}) h56(r) { this.rec('67','Boolean','str_date',r); }
@wire(fw_67_Boolean, {value:['a','b']}) h57(r) { this.rec('67','Boolean','arr_str',r); }
@wire(fw_67_Boolean, {value:{Name:'n'}}) h58(r) { this.rec('67','Boolean','obj_name',r); }
@wire(fw_67_Boolean, {value:'001000000000000AAA'}) h59(r) { this.rec('67','Boolean','str_id18',r); }
@wire(fw_67_Boolean, {value:false}) h60(r) { this.rec('67','Boolean','bool_false',r); }
@wire(fw_67_Date, {value:'x'}) h61(r) { this.rec('67','Date','str_x',r); }
@wire(fw_67_Date, {value:42}) h62(r) { this.rec('67','Date','num_42',r); }
@wire(fw_67_Date, {value:true}) h63(r) { this.rec('67','Date','bool_true',r); }
@wire(fw_67_Date, {value:4.5}) h64(r) { this.rec('67','Date','num_frac',r); }
@wire(fw_67_Date, {value:'42'}) h65(r) { this.rec('67','Date','str_42',r); }
@wire(fw_67_Date, {value:'2024-01-02'}) h66(r) { this.rec('67','Date','str_date',r); }
@wire(fw_67_Date, {value:['a','b']}) h67(r) { this.rec('67','Date','arr_str',r); }
@wire(fw_67_Date, {value:{Name:'n'}}) h68(r) { this.rec('67','Date','obj_name',r); }
@wire(fw_67_Date, {value:'001000000000000AAA'}) h69(r) { this.rec('67','Date','str_id18',r); }
@wire(fw_67_Date, {value:false}) h70(r) { this.rec('67','Date','bool_false',r); }
@wire(fw_67_Datetime, {value:'x'}) h71(r) { this.rec('67','Datetime','str_x',r); }
@wire(fw_67_Datetime, {value:42}) h72(r) { this.rec('67','Datetime','num_42',r); }
@wire(fw_67_Datetime, {value:true}) h73(r) { this.rec('67','Datetime','bool_true',r); }
@wire(fw_67_Datetime, {value:4.5}) h74(r) { this.rec('67','Datetime','num_frac',r); }
@wire(fw_67_Datetime, {value:'42'}) h75(r) { this.rec('67','Datetime','str_42',r); }
@wire(fw_67_Datetime, {value:'2024-01-02'}) h76(r) { this.rec('67','Datetime','str_date',r); }
@wire(fw_67_Datetime, {value:['a','b']}) h77(r) { this.rec('67','Datetime','arr_str',r); }
@wire(fw_67_Datetime, {value:{Name:'n'}}) h78(r) { this.rec('67','Datetime','obj_name',r); }
@wire(fw_67_Datetime, {value:'001000000000000AAA'}) h79(r) { this.rec('67','Datetime','str_id18',r); }
@wire(fw_67_Datetime, {value:false}) h80(r) { this.rec('67','Datetime','bool_false',r); }
@wire(fw_67_Id, {value:'x'}) h81(r) { this.rec('67','Id','str_x',r); }
@wire(fw_67_Id, {value:42}) h82(r) { this.rec('67','Id','num_42',r); }
@wire(fw_67_Id, {value:true}) h83(r) { this.rec('67','Id','bool_true',r); }
@wire(fw_67_Id, {value:4.5}) h84(r) { this.rec('67','Id','num_frac',r); }
@wire(fw_67_Id, {value:'42'}) h85(r) { this.rec('67','Id','str_42',r); }
@wire(fw_67_Id, {value:'2024-01-02'}) h86(r) { this.rec('67','Id','str_date',r); }
@wire(fw_67_Id, {value:['a','b']}) h87(r) { this.rec('67','Id','arr_str',r); }
@wire(fw_67_Id, {value:{Name:'n'}}) h88(r) { this.rec('67','Id','obj_name',r); }
@wire(fw_67_Id, {value:'001000000000000AAA'}) h89(r) { this.rec('67','Id','str_id18',r); }
@wire(fw_67_Id, {value:false}) h90(r) { this.rec('67','Id','bool_false',r); }
@wire(fw_67_ListString, {value:'x'}) h91(r) { this.rec('67','List<String>','str_x',r); }
@wire(fw_67_ListString, {value:42}) h92(r) { this.rec('67','List<String>','num_42',r); }
@wire(fw_67_ListString, {value:true}) h93(r) { this.rec('67','List<String>','bool_true',r); }
@wire(fw_67_ListString, {value:4.5}) h94(r) { this.rec('67','List<String>','num_frac',r); }
@wire(fw_67_ListString, {value:'42'}) h95(r) { this.rec('67','List<String>','str_42',r); }
@wire(fw_67_ListString, {value:'2024-01-02'}) h96(r) { this.rec('67','List<String>','str_date',r); }
@wire(fw_67_ListString, {value:['a','b']}) h97(r) { this.rec('67','List<String>','arr_str',r); }
@wire(fw_67_ListString, {value:{Name:'n'}}) h98(r) { this.rec('67','List<String>','obj_name',r); }
@wire(fw_67_ListString, {value:'001000000000000AAA'}) h99(r) { this.rec('67','List<String>','str_id18',r); }
@wire(fw_67_ListString, {value:false}) h100(r) { this.rec('67','List<String>','bool_false',r); }
@wire(fw_67_Object, {value:'x'}) h101(r) { this.rec('67','Object','str_x',r); }
@wire(fw_67_Object, {value:42}) h102(r) { this.rec('67','Object','num_42',r); }
@wire(fw_67_Object, {value:true}) h103(r) { this.rec('67','Object','bool_true',r); }
@wire(fw_67_Object, {value:4.5}) h104(r) { this.rec('67','Object','num_frac',r); }
@wire(fw_67_Object, {value:'42'}) h105(r) { this.rec('67','Object','str_42',r); }
@wire(fw_67_Object, {value:'2024-01-02'}) h106(r) { this.rec('67','Object','str_date',r); }
@wire(fw_67_Object, {value:['a','b']}) h107(r) { this.rec('67','Object','arr_str',r); }
@wire(fw_67_Object, {value:{Name:'n'}}) h108(r) { this.rec('67','Object','obj_name',r); }
@wire(fw_67_Object, {value:'001000000000000AAA'}) h109(r) { this.rec('67','Object','str_id18',r); }
@wire(fw_67_Object, {value:false}) h110(r) { this.rec('67','Object','bool_false',r); }
@wire(fw_67_Account, {value:'x'}) h111(r) { this.rec('67','Account','str_x',r); }
@wire(fw_67_Account, {value:42}) h112(r) { this.rec('67','Account','num_42',r); }
@wire(fw_67_Account, {value:true}) h113(r) { this.rec('67','Account','bool_true',r); }
@wire(fw_67_Account, {value:4.5}) h114(r) { this.rec('67','Account','num_frac',r); }
@wire(fw_67_Account, {value:'42'}) h115(r) { this.rec('67','Account','str_42',r); }
@wire(fw_67_Account, {value:'2024-01-02'}) h116(r) { this.rec('67','Account','str_date',r); }
@wire(fw_67_Account, {value:['a','b']}) h117(r) { this.rec('67','Account','arr_str',r); }
@wire(fw_67_Account, {value:{Name:'n'}}) h118(r) { this.rec('67','Account','obj_name',r); }
@wire(fw_67_Account, {value:'001000000000000AAA'}) h119(r) { this.rec('67','Account','str_id18',r); }
@wire(fw_67_Account, {value:false}) h120(r) { this.rec('67','Account','bool_false',r); }
@wire(fw_67_SObject, {value:'x'}) h121(r) { this.rec('67','SObject','str_x',r); }
@wire(fw_67_SObject, {value:42}) h122(r) { this.rec('67','SObject','num_42',r); }
@wire(fw_67_SObject, {value:true}) h123(r) { this.rec('67','SObject','bool_true',r); }
@wire(fw_67_SObject, {value:4.5}) h124(r) { this.rec('67','SObject','num_frac',r); }
@wire(fw_67_SObject, {value:'42'}) h125(r) { this.rec('67','SObject','str_42',r); }
@wire(fw_67_SObject, {value:'2024-01-02'}) h126(r) { this.rec('67','SObject','str_date',r); }
@wire(fw_67_SObject, {value:['a','b']}) h127(r) { this.rec('67','SObject','arr_str',r); }
@wire(fw_67_SObject, {value:{Name:'n'}}) h128(r) { this.rec('67','SObject','obj_name',r); }
@wire(fw_67_SObject, {value:'001000000000000AAA'}) h129(r) { this.rec('67','SObject','str_id18',r); }
@wire(fw_67_SObject, {value:false}) h130(r) { this.rec('67','SObject','bool_false',r); }
@wire(fw_54_String, {value:'x'}) h131(r) { this.rec('54','String','str_x',r); }
@wire(fw_54_String, {value:42}) h132(r) { this.rec('54','String','num_42',r); }
@wire(fw_54_String, {value:true}) h133(r) { this.rec('54','String','bool_true',r); }
@wire(fw_54_String, {value:4.5}) h134(r) { this.rec('54','String','num_frac',r); }
@wire(fw_54_String, {value:'42'}) h135(r) { this.rec('54','String','str_42',r); }
@wire(fw_54_String, {value:'2024-01-02'}) h136(r) { this.rec('54','String','str_date',r); }
@wire(fw_54_String, {value:['a','b']}) h137(r) { this.rec('54','String','arr_str',r); }
@wire(fw_54_String, {value:{Name:'n'}}) h138(r) { this.rec('54','String','obj_name',r); }
@wire(fw_54_String, {value:'001000000000000AAA'}) h139(r) { this.rec('54','String','str_id18',r); }
@wire(fw_54_String, {value:false}) h140(r) { this.rec('54','String','bool_false',r); }
@wire(fw_54_Integer, {value:'x'}) h141(r) { this.rec('54','Integer','str_x',r); }
@wire(fw_54_Integer, {value:42}) h142(r) { this.rec('54','Integer','num_42',r); }
@wire(fw_54_Integer, {value:true}) h143(r) { this.rec('54','Integer','bool_true',r); }
@wire(fw_54_Integer, {value:4.5}) h144(r) { this.rec('54','Integer','num_frac',r); }
@wire(fw_54_Integer, {value:'42'}) h145(r) { this.rec('54','Integer','str_42',r); }
@wire(fw_54_Integer, {value:'2024-01-02'}) h146(r) { this.rec('54','Integer','str_date',r); }
@wire(fw_54_Integer, {value:['a','b']}) h147(r) { this.rec('54','Integer','arr_str',r); }
@wire(fw_54_Integer, {value:{Name:'n'}}) h148(r) { this.rec('54','Integer','obj_name',r); }
@wire(fw_54_Integer, {value:'001000000000000AAA'}) h149(r) { this.rec('54','Integer','str_id18',r); }
@wire(fw_54_Integer, {value:false}) h150(r) { this.rec('54','Integer','bool_false',r); }
@wire(fw_54_Long, {value:'x'}) h151(r) { this.rec('54','Long','str_x',r); }
@wire(fw_54_Long, {value:42}) h152(r) { this.rec('54','Long','num_42',r); }
@wire(fw_54_Long, {value:true}) h153(r) { this.rec('54','Long','bool_true',r); }
@wire(fw_54_Long, {value:4.5}) h154(r) { this.rec('54','Long','num_frac',r); }
@wire(fw_54_Long, {value:'42'}) h155(r) { this.rec('54','Long','str_42',r); }
@wire(fw_54_Long, {value:'2024-01-02'}) h156(r) { this.rec('54','Long','str_date',r); }
@wire(fw_54_Long, {value:['a','b']}) h157(r) { this.rec('54','Long','arr_str',r); }
@wire(fw_54_Long, {value:{Name:'n'}}) h158(r) { this.rec('54','Long','obj_name',r); }
@wire(fw_54_Long, {value:'001000000000000AAA'}) h159(r) { this.rec('54','Long','str_id18',r); }
@wire(fw_54_Long, {value:false}) h160(r) { this.rec('54','Long','bool_false',r); }
@wire(fw_54_Decimal, {value:'x'}) h161(r) { this.rec('54','Decimal','str_x',r); }
@wire(fw_54_Decimal, {value:42}) h162(r) { this.rec('54','Decimal','num_42',r); }
@wire(fw_54_Decimal, {value:true}) h163(r) { this.rec('54','Decimal','bool_true',r); }
@wire(fw_54_Decimal, {value:4.5}) h164(r) { this.rec('54','Decimal','num_frac',r); }
@wire(fw_54_Decimal, {value:'42'}) h165(r) { this.rec('54','Decimal','str_42',r); }
@wire(fw_54_Decimal, {value:'2024-01-02'}) h166(r) { this.rec('54','Decimal','str_date',r); }
@wire(fw_54_Decimal, {value:['a','b']}) h167(r) { this.rec('54','Decimal','arr_str',r); }
@wire(fw_54_Decimal, {value:{Name:'n'}}) h168(r) { this.rec('54','Decimal','obj_name',r); }
@wire(fw_54_Decimal, {value:'001000000000000AAA'}) h169(r) { this.rec('54','Decimal','str_id18',r); }
@wire(fw_54_Decimal, {value:false}) h170(r) { this.rec('54','Decimal','bool_false',r); }
@wire(fw_54_Double, {value:'x'}) h171(r) { this.rec('54','Double','str_x',r); }
@wire(fw_54_Double, {value:42}) h172(r) { this.rec('54','Double','num_42',r); }
@wire(fw_54_Double, {value:true}) h173(r) { this.rec('54','Double','bool_true',r); }
@wire(fw_54_Double, {value:4.5}) h174(r) { this.rec('54','Double','num_frac',r); }
@wire(fw_54_Double, {value:'42'}) h175(r) { this.rec('54','Double','str_42',r); }
@wire(fw_54_Double, {value:'2024-01-02'}) h176(r) { this.rec('54','Double','str_date',r); }
@wire(fw_54_Double, {value:['a','b']}) h177(r) { this.rec('54','Double','arr_str',r); }
@wire(fw_54_Double, {value:{Name:'n'}}) h178(r) { this.rec('54','Double','obj_name',r); }
@wire(fw_54_Double, {value:'001000000000000AAA'}) h179(r) { this.rec('54','Double','str_id18',r); }
@wire(fw_54_Double, {value:false}) h180(r) { this.rec('54','Double','bool_false',r); }
@wire(fw_54_Boolean, {value:'x'}) h181(r) { this.rec('54','Boolean','str_x',r); }
@wire(fw_54_Boolean, {value:42}) h182(r) { this.rec('54','Boolean','num_42',r); }
@wire(fw_54_Boolean, {value:true}) h183(r) { this.rec('54','Boolean','bool_true',r); }
@wire(fw_54_Boolean, {value:4.5}) h184(r) { this.rec('54','Boolean','num_frac',r); }
@wire(fw_54_Boolean, {value:'42'}) h185(r) { this.rec('54','Boolean','str_42',r); }
@wire(fw_54_Boolean, {value:'2024-01-02'}) h186(r) { this.rec('54','Boolean','str_date',r); }
@wire(fw_54_Boolean, {value:['a','b']}) h187(r) { this.rec('54','Boolean','arr_str',r); }
@wire(fw_54_Boolean, {value:{Name:'n'}}) h188(r) { this.rec('54','Boolean','obj_name',r); }
@wire(fw_54_Boolean, {value:'001000000000000AAA'}) h189(r) { this.rec('54','Boolean','str_id18',r); }
@wire(fw_54_Boolean, {value:false}) h190(r) { this.rec('54','Boolean','bool_false',r); }
@wire(fw_54_Date, {value:'x'}) h191(r) { this.rec('54','Date','str_x',r); }
@wire(fw_54_Date, {value:42}) h192(r) { this.rec('54','Date','num_42',r); }
@wire(fw_54_Date, {value:true}) h193(r) { this.rec('54','Date','bool_true',r); }
@wire(fw_54_Date, {value:4.5}) h194(r) { this.rec('54','Date','num_frac',r); }
@wire(fw_54_Date, {value:'42'}) h195(r) { this.rec('54','Date','str_42',r); }
@wire(fw_54_Date, {value:'2024-01-02'}) h196(r) { this.rec('54','Date','str_date',r); }
@wire(fw_54_Date, {value:['a','b']}) h197(r) { this.rec('54','Date','arr_str',r); }
@wire(fw_54_Date, {value:{Name:'n'}}) h198(r) { this.rec('54','Date','obj_name',r); }
@wire(fw_54_Date, {value:'001000000000000AAA'}) h199(r) { this.rec('54','Date','str_id18',r); }
@wire(fw_54_Date, {value:false}) h200(r) { this.rec('54','Date','bool_false',r); }
@wire(fw_54_Datetime, {value:'x'}) h201(r) { this.rec('54','Datetime','str_x',r); }
@wire(fw_54_Datetime, {value:42}) h202(r) { this.rec('54','Datetime','num_42',r); }
@wire(fw_54_Datetime, {value:true}) h203(r) { this.rec('54','Datetime','bool_true',r); }
@wire(fw_54_Datetime, {value:4.5}) h204(r) { this.rec('54','Datetime','num_frac',r); }
@wire(fw_54_Datetime, {value:'42'}) h205(r) { this.rec('54','Datetime','str_42',r); }
@wire(fw_54_Datetime, {value:'2024-01-02'}) h206(r) { this.rec('54','Datetime','str_date',r); }
@wire(fw_54_Datetime, {value:['a','b']}) h207(r) { this.rec('54','Datetime','arr_str',r); }
@wire(fw_54_Datetime, {value:{Name:'n'}}) h208(r) { this.rec('54','Datetime','obj_name',r); }
@wire(fw_54_Datetime, {value:'001000000000000AAA'}) h209(r) { this.rec('54','Datetime','str_id18',r); }
@wire(fw_54_Datetime, {value:false}) h210(r) { this.rec('54','Datetime','bool_false',r); }
@wire(fw_54_Id, {value:'x'}) h211(r) { this.rec('54','Id','str_x',r); }
@wire(fw_54_Id, {value:42}) h212(r) { this.rec('54','Id','num_42',r); }
@wire(fw_54_Id, {value:true}) h213(r) { this.rec('54','Id','bool_true',r); }
@wire(fw_54_Id, {value:4.5}) h214(r) { this.rec('54','Id','num_frac',r); }
@wire(fw_54_Id, {value:'42'}) h215(r) { this.rec('54','Id','str_42',r); }
@wire(fw_54_Id, {value:'2024-01-02'}) h216(r) { this.rec('54','Id','str_date',r); }
@wire(fw_54_Id, {value:['a','b']}) h217(r) { this.rec('54','Id','arr_str',r); }
@wire(fw_54_Id, {value:{Name:'n'}}) h218(r) { this.rec('54','Id','obj_name',r); }
@wire(fw_54_Id, {value:'001000000000000AAA'}) h219(r) { this.rec('54','Id','str_id18',r); }
@wire(fw_54_Id, {value:false}) h220(r) { this.rec('54','Id','bool_false',r); }
@wire(fw_54_ListString, {value:'x'}) h221(r) { this.rec('54','List<String>','str_x',r); }
@wire(fw_54_ListString, {value:42}) h222(r) { this.rec('54','List<String>','num_42',r); }
@wire(fw_54_ListString, {value:true}) h223(r) { this.rec('54','List<String>','bool_true',r); }
@wire(fw_54_ListString, {value:4.5}) h224(r) { this.rec('54','List<String>','num_frac',r); }
@wire(fw_54_ListString, {value:'42'}) h225(r) { this.rec('54','List<String>','str_42',r); }
@wire(fw_54_ListString, {value:'2024-01-02'}) h226(r) { this.rec('54','List<String>','str_date',r); }
@wire(fw_54_ListString, {value:['a','b']}) h227(r) { this.rec('54','List<String>','arr_str',r); }
@wire(fw_54_ListString, {value:{Name:'n'}}) h228(r) { this.rec('54','List<String>','obj_name',r); }
@wire(fw_54_ListString, {value:'001000000000000AAA'}) h229(r) { this.rec('54','List<String>','str_id18',r); }
@wire(fw_54_ListString, {value:false}) h230(r) { this.rec('54','List<String>','bool_false',r); }
@wire(fw_54_Object, {value:'x'}) h231(r) { this.rec('54','Object','str_x',r); }
@wire(fw_54_Object, {value:42}) h232(r) { this.rec('54','Object','num_42',r); }
@wire(fw_54_Object, {value:true}) h233(r) { this.rec('54','Object','bool_true',r); }
@wire(fw_54_Object, {value:4.5}) h234(r) { this.rec('54','Object','num_frac',r); }
@wire(fw_54_Object, {value:'42'}) h235(r) { this.rec('54','Object','str_42',r); }
@wire(fw_54_Object, {value:'2024-01-02'}) h236(r) { this.rec('54','Object','str_date',r); }
@wire(fw_54_Object, {value:['a','b']}) h237(r) { this.rec('54','Object','arr_str',r); }
@wire(fw_54_Object, {value:{Name:'n'}}) h238(r) { this.rec('54','Object','obj_name',r); }
@wire(fw_54_Object, {value:'001000000000000AAA'}) h239(r) { this.rec('54','Object','str_id18',r); }
@wire(fw_54_Object, {value:false}) h240(r) { this.rec('54','Object','bool_false',r); }
@wire(fw_54_Account, {value:'x'}) h241(r) { this.rec('54','Account','str_x',r); }
@wire(fw_54_Account, {value:42}) h242(r) { this.rec('54','Account','num_42',r); }
@wire(fw_54_Account, {value:true}) h243(r) { this.rec('54','Account','bool_true',r); }
@wire(fw_54_Account, {value:4.5}) h244(r) { this.rec('54','Account','num_frac',r); }
@wire(fw_54_Account, {value:'42'}) h245(r) { this.rec('54','Account','str_42',r); }
@wire(fw_54_Account, {value:'2024-01-02'}) h246(r) { this.rec('54','Account','str_date',r); }
@wire(fw_54_Account, {value:['a','b']}) h247(r) { this.rec('54','Account','arr_str',r); }
@wire(fw_54_Account, {value:{Name:'n'}}) h248(r) { this.rec('54','Account','obj_name',r); }
@wire(fw_54_Account, {value:'001000000000000AAA'}) h249(r) { this.rec('54','Account','str_id18',r); }
@wire(fw_54_Account, {value:false}) h250(r) { this.rec('54','Account','bool_false',r); }
@wire(fw_54_SObject, {value:'x'}) h251(r) { this.rec('54','SObject','str_x',r); }
@wire(fw_54_SObject, {value:42}) h252(r) { this.rec('54','SObject','num_42',r); }
@wire(fw_54_SObject, {value:true}) h253(r) { this.rec('54','SObject','bool_true',r); }
@wire(fw_54_SObject, {value:4.5}) h254(r) { this.rec('54','SObject','num_frac',r); }
@wire(fw_54_SObject, {value:'42'}) h255(r) { this.rec('54','SObject','str_42',r); }
@wire(fw_54_SObject, {value:'2024-01-02'}) h256(r) { this.rec('54','SObject','str_date',r); }
@wire(fw_54_SObject, {value:['a','b']}) h257(r) { this.rec('54','SObject','arr_str',r); }
@wire(fw_54_SObject, {value:{Name:'n'}}) h258(r) { this.rec('54','SObject','obj_name',r); }
@wire(fw_54_SObject, {value:'001000000000000AAA'}) h259(r) { this.rec('54','SObject','str_id18',r); }
@wire(fw_54_SObject, {value:false}) h260(r) { this.rec('54','SObject','bool_false',r); }
rec(api,type,c,r) { if (r.error) { const b=r.error.body||{}; this.rows.push(['wire',api,type,c,'error',(b.message||(b.pageErrors&&b.pageErrors[0]&&b.pageErrors[0].message)||r.error.message||String(r.error)),b.exceptionType||'']); } else if (r.data!==undefined) { this.rows.push(['wire',api,type,c,'ok',String(r.data),'']); } else return; if (this.rows.length>=260) this.flush(); }
connectedCallback() { setTimeout(()=>this.flush(), 90000); }
async flush() { if (this.saved) return; this.saved=true; const rows=this.rows; for (let i=0;i<rows.length;i+=40) { await save({label:'OvW67_'+i,value:JSON.stringify(rows.slice(i,i+40))}); } await save({label:'OvDoneW67',value:String(rows.length)+'/260'}); this.result='saved'; }}
