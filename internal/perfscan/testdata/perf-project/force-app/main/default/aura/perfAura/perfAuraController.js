({
    load: function(component) {
        var action = component.get("c.uncachedAccounts");
        $A.enqueueAction(action);
    }
})
