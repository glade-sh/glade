@IsTest private class CRT625d2c7ff4c537C001 {
virtual class BaseValue {} virtual class NarrowValue extends BaseValue {} class LeafValue extends NarrowValue {} class OverloadProbe { public String choose(NarrowValue left,BaseValue right){return 'narrow-base';} public String choose(BaseValue left,LeafValue right){return 'base-leaf';} } 
@IsTest static void C001(){
Object r=new OverloadProbe().choose(new NarrowValue(),new LeafValue()); 
System.assert(false,'P|C001|'+String.valueOf(r));
}
}
