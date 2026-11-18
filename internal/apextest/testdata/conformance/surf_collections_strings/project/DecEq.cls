@IsTest private class DecEq {
 @IsTest static void t() {
 String p = 'dec';
 try { p += '|l3_30=' + (new List<Decimal>{3} == new List<Decimal>{3.0}); } catch (Exception x) { p += '|l3_30=EX:' + x.getMessage(); }
 try { p += '|l15_150=' + (new List<Decimal>{1.5} == new List<Decimal>{1.50}); } catch (Exception x) { p += '|l15_150=EX:' + x.getMessage(); }
 try { p += '|l15_15=' + (new List<Decimal>{1.5} == new List<Decimal>{1.5}); } catch (Exception x) { p += '|l15_15=EX:' + x.getMessage(); }
 try { p += '|l3_30_equals=' + (new List<Decimal>{3}.equals(new List<Decimal>{3.0})); } catch (Exception x) { p += '|l3_30_equals=EX:' + x.getMessage(); }
 try { p += '|l15_150_equals=' + (new List<Decimal>{1.5}.equals(new List<Decimal>{1.50})); } catch (Exception x) { p += '|l15_150_equals=EX:' + x.getMessage(); }
 try { p += '|lo15_150=' + (new List<Object>{1.5} == new List<Object>{1.50}); } catch (Exception x) { p += '|lo15_150=EX:' + x.getMessage(); }
 try { p += '|lo15_150_vs=' + (new List<Object>{Decimal.valueOf('1.5')} == new List<Object>{Decimal.valueOf('1.50')}); } catch (Exception x) { p += '|lo15_150_vs=EX:' + x.getMessage(); }
 try { p += '|lo_int_dec=' + (new List<Object>{3} == new List<Object>{3.0}); } catch (Exception x) { p += '|lo_int_dec=EX:' + x.getMessage(); }
 try { p += '|nested=' + (new List<List<Decimal>>{new List<Decimal>{1.5}} == new List<List<Decimal>>{new List<Decimal>{1.50}}); } catch (Exception x) { p += '|nested=EX:' + x.getMessage(); }
 try { p += '|nested3=' + (new List<List<Decimal>>{new List<Decimal>{3}} == new List<List<Decimal>>{new List<Decimal>{3.0}}); } catch (Exception x) { p += '|nested3=EX:' + x.getMessage(); }
 try { p += '|set=' + (new Set<Decimal>{1.5} == new Set<Decimal>{1.50}); } catch (Exception x) { p += '|set=EX:' + x.getMessage(); }
 try { p += '|map_val=' + (new Map<String,Decimal>{'a'=>1.5} == new Map<String,Decimal>{'a'=>1.50}); } catch (Exception x) { p += '|map_val=EX:' + x.getMessage(); }
 try { p += '|l_contains=' + (new List<Decimal>{1.5}.contains(1.50)); } catch (Exception x) { p += '|l_contains=EX:' + x.getMessage(); }
 try { p += '|l_indexof=' + (new List<Decimal>{1.5}.indexOf(1.50)); } catch (Exception x) { p += '|l_indexof=EX:' + x.getMessage(); }
 try { p += '|dec_eq=' + ((Decimal)1.5 == (Decimal)1.50); } catch (Exception x) { p += '|dec_eq=EX:' + x.getMessage(); }
 try { p += '|dbl_l=' + (new List<Double>{1.5} == new List<Double>{1.50}); } catch (Exception x) { p += '|dbl_l=EX:' + x.getMessage(); }
 try { p += '|l_sobj=' + (new List<Account>{new Account(AnnualRevenue=1.5)} == new List<Account>{new Account(AnnualRevenue=1.50)}); } catch (Exception x) { p += '|l_sobj=EX:' + x.getMessage(); }
 try { p += '|neg_zero=' + (new List<Decimal>{0} == new List<Decimal>{0.00}); } catch (Exception x) { p += '|neg_zero=EX:' + x.getMessage(); }
 System.assert(false, p);
 }
}
