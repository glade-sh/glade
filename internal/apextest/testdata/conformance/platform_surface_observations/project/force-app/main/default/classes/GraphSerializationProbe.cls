public class GraphSerializationProbe {
 public abstract class Base implements Queueable,Comparable {public void execute(QueueableContext c){} public Integer compareTo(Object x){return 0;}}
 public class SelfNode extends Base {public SelfNode self;}
 public class TreeNode {public String label;public TreeNode parent;public List<TreeNode> children;}
 public interface Marker {String value();}
 public class Leaf extends Base implements Marker {public String label='leaf';public String value(){return label;}}
 public class InterfaceHolder extends Base {public Comparable held;}
 public class CustomInterfaceHolder {public Marker held;}
 public class AbstractHolder {public Base held;}
 public class ConcreteHolder {public Leaf held;}
 public class TransientHolder {public transient Comparable held;}
 private static Map<String,Object> failure(Exception e){return new Map<String,Object>{'type'=>e.getTypeName(),'message'=>e.getMessage().startsWith('Cycle detected.')?'Cycle detected.':e.getMessage()};}
 public static Map<String,Object> observe(){
  Map<String,Object> r=new Map<String,Object>();SelfNode cycle=new SelfNode();cycle.self=cycle;
  try{r.put('selfCycle',new Map<String,Object>{'json'=>JSON.serialize(cycle)});}catch(Exception e){r.put('selfCycle',failure(e));}
  try{r.put('prettyCycle',new Map<String,Object>{'json'=>JSON.serializePretty(cycle)});}catch(Exception e){r.put('prettyCycle',failure(e));}
  try{r.put('suppressedCycle',new Map<String,Object>{'json'=>JSON.serialize(cycle,true)});}catch(Exception e){r.put('suppressedCycle',failure(e));}
  TreeNode parent=new TreeNode();parent.label='root';TreeNode child=new TreeNode();child.label='child';child.parent=parent;parent.children=new List<TreeNode>{child};
  try{r.put('treeCycle',new Map<String,Object>{'json'=>JSON.serialize(new Map<String,TreeNode>{'root'=>parent})});}catch(Exception e){r.put('treeCycle',failure(e));}
  Leaf leaf=new Leaf();InterfaceHolder holder=new InterfaceHolder();holder.held=leaf;
  try{JSON.deserialize(JSON.serialize(holder),InterfaceHolder.class);r.put('interfaceField',new Map<String,Object>{'accepted'=>true});}catch(Exception e){r.put('interfaceField',failure(e));}
  CustomInterfaceHolder custom=new CustomInterfaceHolder();custom.held=leaf;
  try{JSON.deserialize(JSON.serialize(custom),CustomInterfaceHolder.class);r.put('customInterfaceField',new Map<String,Object>{'accepted'=>true});}catch(Exception e){r.put('customInterfaceField',failure(e));}
  AbstractHolder abstractHolder=new AbstractHolder();abstractHolder.held=leaf;
  try{JSON.deserialize(JSON.serialize(abstractHolder),AbstractHolder.class);r.put('abstractField',new Map<String,Object>{'accepted'=>true});}catch(Exception e){r.put('abstractField',failure(e));}
  InterfaceHolder nullHolder=(InterfaceHolder)JSON.deserialize(JSON.serialize(new InterfaceHolder()),InterfaceHolder.class);r.put('nullInterfaceControl',nullHolder.held==null);
  TransientHolder transientHolder=new TransientHolder();transientHolder.held=leaf;TransientHolder transientCopy=(TransientHolder)JSON.deserialize(JSON.serialize(transientHolder),TransientHolder.class);r.put('transientControl',transientCopy.held==null);
  ConcreteHolder concrete=new ConcreteHolder();concrete.held=leaf;ConcreteHolder concreteCopy=(ConcreteHolder)JSON.deserialize(JSON.serialize(concrete),ConcreteHolder.class);r.put('concreteControl',concreteCopy.held.label);
  List<Object> shared=(List<Object>)JSON.deserializeUntyped(JSON.serialize(new List<Leaf>{leaf,leaf}));r.put('sharedAliasControl',shared.size()==2&&((Map<String,Object>)shared[0]).get('label')=='leaf'&&((Map<String,Object>)shared[1]).get('label')=='leaf');
  Map<String,Object> acyclic=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(new SelfNode()));r.put('acyclicControl',acyclic.containsKey('self')&&acyclic.get('self')==null);
  return r;
 }
}
