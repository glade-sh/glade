import { LightningElement } from 'lwc';
import { navigate, NavigationContext } from 'lightning/navigation';
export default class Probe extends LightningElement { result='pending'; connectedCallback(){try {this.result=JSON.stringify({navigate:typeof navigate,context:typeof NavigationContext});} catch(e){this.result=JSON.stringify({error:String(e)});} } }