public class LargeSearchWindow{public static String summary(String query){
List<List<SObject>> groups=Search.query(query);String text='';
for(List<SObject> rows:groups){
String f=rows.isEmpty()||rows[0] instanceof Account?'Name':'LastName';
String first=rows.isEmpty()?'':((String)rows[0].get(f)).substring(13);
String last=rows.isEmpty()?'':((String)rows[rows.size()-1].get(f)).substring(13);
text+='['+rows.size()+':'+first+':'+last+']';}return text;}
public static void seed(){List<Account> rows=new List<Account>();
for(Integer i=0;i<2002;i++){String n=String.valueOf(i).leftPad(4,'0');rows.add(new Account(Name='Lindenwindow '+n));}
insert rows;insert new List<Contact>{new Contact(LastName='Lindenwindow K'),new Contact(LastName='Lindenwindow M')};}}
