public class Ml2Probe62 {
  static String vis(String s){ return s==null?'null':s.replace('\n','~').replace('\r','^').replace('\t','>').replace(' ','.'); }
  public static String run(){
    List<String> o=new List<String>();
    String a='''
    x
      y
  ''';
    o.add('MLa ['+vis(a)+']');
    String b='''
    x
    y''';
    o.add('MLb ['+vis(b)+']');
    String c='''
	tabbed
	  more
	''';
    o.add('MLc ['+vis(c)+']');
    String d='''
one

two


three
''';
    o.add('MLd ['+vis(d)+']');
    String e='''
q \\ b \' s \" d A e
''';
    o.add('MLe ['+vis(e)+']');
    String f='''
cont \
inued
''';
    o.add('MLf ['+vis(f)+']');
    String g='''
  keep
  end  ''';
    o.add('MLg ['+vis(g)+']');
    String h='''
 "dq" and 'sq' and ''two'' and """
''';
    o.add('MLh ['+vis(h)+']');
    String i='''
      deep
    shallow
        deeper
      ''';
    o.add('MLi ['+vis(i)+']');
    String j='''
  a

  b
  ''';
    o.add('MLj ['+vis(j)+']');
    String k='''
x
''' + '''
y
''';
    o.add('MLk ['+vis(k)+']');
    String l='''
   z''';
    o.add('MLl ['+vis(l)+'] len='+l.length());
    String m='''
  a \t b \r c \n d e \b f \f g
''';
    o.add('MLm ['+vis(m)+']');
    String n='''
  tail space\s
''';
    o.add('MLn ['+vis(n)+']');
    return String.join(o,' ## ');
  }
}
