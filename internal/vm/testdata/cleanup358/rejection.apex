Map<String, Schema.SObjectType> view = Schema.getGlobalDescribe();
Integer beforeSize = view.size();
Schema.SObjectType original = view.get('Account');
Schema.SObjectType contact = view.get('Contact');
try {
    view.put('Account', contact);
    System.debug('C358_REJECT=NO_EXCEPTION');
} catch (Exception ex) {
    System.debug('C358_REJECT=' + ex.getTypeName() + ':' + ex.getMessage());
}
System.debug('C358_BOUNDARY=' + (original != null) + ',' + (contact != null) + ',' + (view.size() == beforeSize) + ',' + (view.get('Account') == original) + ',' + (view.get('not_a_real_object_358') == null));
System.debug('C358_FRESH=' + (Schema.getGlobalDescribe().get('Account') == original));
