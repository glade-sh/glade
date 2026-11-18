System.debug('N3|META|locale='+UserInfo.getLocale()+'|tz='+UserInfo.getTimeZone().getID());
Integer imax=2147483647; Integer imin=-2147483647-1;
Long lmax=9223372036854775807L; Long lmin=-9223372036854775807L-1;
Integer ip1=imax+1; System.debug('N3|EDGE|int_max_plus_1|'+String.valueOf(ip1));
System.debug('N3|EDGE|int_max_times_2|'+String.valueOf(imax*2));
System.debug('N3|EDGE|int_min_neg|'+String.valueOf(-imin));
System.debug('N3|EDGE|int_abs_min|'+String.valueOf(Math.abs(imin)));
Long lp1=lmax+1; System.debug('N3|EDGE|long_max_plus_1|'+String.valueOf(lp1));
System.debug('N3|EDGE|long_max_times_2|'+String.valueOf(lmax*2));
System.debug('N3|EDGE|long_min_neg|'+String.valueOf(-lmin));
try{Integer x=Integer.valueOf('2147483648');System.debug('N3|EDGE|int_overflow_parse|'+String.valueOf(x));}catch(Exception e){System.debug('N3|EDGE|int_overflow_parse|ERR|'+e.getTypeName()+'|'+e.getMessage());}
try{Long x=Long.valueOf('9223372036854775808');System.debug('N3|EDGE|long_overflow_parse|'+String.valueOf(x));}catch(Exception e){System.debug('N3|EDGE|long_overflow_parse|ERR|'+e.getTypeName()+'|'+e.getMessage());}
try{Decimal x=Decimal.valueOf('1e40');System.debug('N3|EDGE|dec_exp_40|'+String.valueOf(x));}catch(Exception e){System.debug('N3|EDGE|dec_exp_40|ERR|'+e.getTypeName()+'|'+e.getMessage());}
try{Decimal x=Decimal.valueOf('0.000000000000000000000000000000000001');System.debug('N3|EDGE|dec_too_small|'+String.valueOf(x)+'|'+x.scale());}catch(Exception e){System.debug('N3|EDGE|dec_too_small|ERR|'+e.getTypeName()+'|'+e.getMessage());}
try{Decimal x=Decimal.valueOf('12345678901234567890123456789012345');System.debug('N3|EDGE|dec_35_digits|'+String.valueOf(x)+'|'+x.precision());}catch(Exception e){System.debug('N3|EDGE|dec_35_digits|ERR|'+e.getTypeName()+'|'+e.getMessage());}
Decimal one=1; Decimal three=3; Decimal zero=0;
Decimal ot=one/three; System.debug('N3|EDGE|one_div_three|'+String.valueOf(ot)+'|'+ot.scale());
Decimal tt=2/three; System.debug('N3|EDGE|two_div_three|'+String.valueOf(2.0/3)+'|'+tt.scale());
try{Decimal z=one/zero;System.debug('N3|EDGE|div_zero_dec|'+String.valueOf(z));}catch(Exception e){System.debug('N3|EDGE|div_zero_dec|ERR|'+e.getTypeName()+'|'+e.getMessage());}
try{Decimal z=zero/zero;System.debug('N3|EDGE|zero_div_zero|'+String.valueOf(z));}catch(Exception e){System.debug('N3|EDGE|zero_div_zero|ERR|'+e.getTypeName()+'|'+e.getMessage());}
Double d01=0.1; Decimal c01=Decimal.valueOf(d01);
System.debug('N3|EDGE|double_01_to_dec|'+String.valueOf(c01)+'|'+c01.scale());
Double dtiny=Double.valueOf('1E-300'); Decimal ctiny=Decimal.valueOf(dtiny);
System.debug('N3|EDGE|double_tiny_to_dec|'+String.valueOf(ctiny)+'|'+ctiny.scale());
Double dbig=Double.valueOf('1.7976931348623157E308');
try{Decimal cbig=Decimal.valueOf(dbig);System.debug('N3|EDGE|double_max_to_dec|'+String.valueOf(cbig)+'|'+cbig.scale());}catch(Exception e){System.debug('N3|EDGE|double_max_to_dec|ERR|'+e.getTypeName()+'|'+e.getMessage());}
Double dinf=Double.valueOf('1E308')*10;
try{System.debug('N3|EDGE|double_inf|'+String.valueOf(dinf)+'|'+JSON.serialize(dinf));}catch(Exception e){System.debug('N3|EDGE|double_inf|ERR|'+e.getTypeName()+'|'+e.getMessage());}
try{Decimal cinf=Decimal.valueOf(dinf);System.debug('N3|EDGE|double_inf_to_dec|'+String.valueOf(cinf));}catch(Exception e){System.debug('N3|EDGE|double_inf_to_dec|ERR|'+e.getTypeName()+'|'+e.getMessage());}
Double nz=-0.0; try{Decimal neg=Decimal.valueOf(nz);System.debug('N3|EDGE|decimal_neg_zero|'+String.valueOf(neg)+'|'+neg.scale());}catch(Exception e){System.debug('N3|EDGE|decimal_neg_zero|ERR|'+e.getTypeName()+'|'+e.getMessage());}
