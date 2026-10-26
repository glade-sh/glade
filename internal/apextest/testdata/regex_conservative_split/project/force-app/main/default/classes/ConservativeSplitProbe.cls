public class ConservativeSplitProbe {
 public static Map<String,Object> observe(String source,String separator) {
  String text='L'+separator+'R'+separator+'T';Map<String,Object> rows=new Map<String,Object>();
  try {rows.put('String.split/default',text.split(source));} catch(Exception e){rows.put('String.split/default',e.getTypeName());}
  try {rows.put('String.split/negative',text.split(source,-1));} catch(Exception e){rows.put('String.split/negative',e.getTypeName());}
  try {rows.put('String.split/one',text.split(source,1));} catch(Exception e){rows.put('String.split/one',e.getTypeName());}
  try {rows.put('String.split/positive',text.split(source,2));} catch(Exception e){rows.put('String.split/positive',e.getTypeName());}
  try {rows.put('Pattern.split/default',Pattern.compile(source).split(text));} catch(Exception e){rows.put('Pattern.split/default',e.getTypeName());}
  try {rows.put('Pattern.split/negative',Pattern.compile(source).split(text,-1));} catch(Exception e){rows.put('Pattern.split/negative',e.getTypeName());}
  try {rows.put('Pattern.split/one',Pattern.compile(source).split(text,1));} catch(Exception e){rows.put('Pattern.split/one',e.getTypeName());}
  try {rows.put('Pattern.split/positive',Pattern.compile(source).split(text,2));} catch(Exception e){rows.put('Pattern.split/positive',e.getTypeName());}
  try {rows.put('Matcher.find',Pattern.compile(source).matcher(text).find());} catch(Exception e){rows.put('Matcher.find',e.getTypeName());}
  return rows;
 }
}
