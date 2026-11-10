@IsTest private class AccessorHelperTest62 {
 static Integer setterVisits=0;
 static String finiteSetter { get; set { setterVisits++; if (setterVisits==1) setterHelper(value); finiteSetter=value; } }
 static void setterHelper(String data) { finiteSetter=data; }
 static String recursiveSetter { get; set { recursiveSetterHelper(value); recursiveSetter=value; } }
 static void recursiveSetterHelper(String data) { recursiveSetter=data; }
 class InstanceProbe {
  public Integer visits=0;
  public String left { get { visits++; if (visits==1) { String currentValue=right; } return 'left'; } set; }
  public String right { get { return left; } set; }
  public String recursiveLeft { get { return recursiveRight; } set; }
  public String recursiveRight { get { return recursiveLeft; } set; }
 }
 @IsTest static void setterFinite() { finiteSetter='x'; System.assertEquals(2,setterVisits); System.assertEquals('x',finiteSetter); }
 @IsTest static void setterRecursive() { recursiveSetter='x'; }
 @IsTest static void getterFinite() { InstanceProbe obj=new InstanceProbe(); System.assertEquals('left',obj.left); System.assertEquals(2,obj.visits); }
 @IsTest static void getterRecursive() { InstanceProbe obj=new InstanceProbe(); String currentValue=obj.recursiveLeft; }
}
