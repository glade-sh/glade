<aura:application controller="LtProbe" access="GLOBAL">
  <aura:handler name="init" value="{!this}" action="{!c.go}"/>
  <aura:attribute name="out" type="String" default="running"/>
  <pre id="out">{!v.out}</pre>
</aura:application>
