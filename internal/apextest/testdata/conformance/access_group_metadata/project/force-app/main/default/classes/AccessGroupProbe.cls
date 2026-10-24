public class AccessGroupProbe {
    public class State {
        public Integer roleGroupCount;
        public List<String> roleGroupTypes;
        public Integer internalRoleGroupCount;
        public Integer memberQueryCount;
        public Integer filteredMemberCount;
        public List<String> groupMemberChildRelationships;
        public List<String> selectedTypePicklistValues;
    }

    public static Map<String, Object> observe() {
        Savepoint point = Database.setSavepoint();
        try {
            Map<String, Object> result = snapshot(prepare());
            Database.rollback(point);
            return result;
        } catch (Exception e) {
            Database.rollback(point);
            throw e;
        }
    }

    private static State prepare() {
        State state = new State();
        UserRole role = new UserRole(Name = 'Probe Role', DeveloperName = 'Probe_Role');
        insert role;

            List<Group> roleGroups = [
                SELECT Id, Type FROM Group WHERE RelatedId = :role.Id ORDER BY Type
            ];
            state.roleGroupTypes = new List<String>();
            state.roleGroupCount = roleGroups.size();
            state.internalRoleGroupCount = 0;
            for (Group roleGroup : roleGroups) {
                state.roleGroupTypes.add(roleGroup.Type);
                if (roleGroup.Type == 'RoleAndSubordinatesInternal') {
                    state.internalRoleGroupCount++;
                }
            }

            Group regular = new Group(
                Name = 'Probe Group',
                DeveloperName = 'Probe_Group',
                Type = 'Regular'
            );
            insert regular;
            insert new GroupMember(GroupId = regular.Id, UserOrGroupId = UserInfo.getUserId());

            Group projected = [
                SELECT Id, (SELECT Id, GroupId, UserOrGroupId FROM GroupMembers)
                FROM Group WHERE Id = :regular.Id
            ];
            state.memberQueryCount = projected.GroupMembers.size();
            state.filteredMemberCount = [
                SELECT COUNT() FROM GroupMember
                WHERE GroupId = :regular.Id AND Group.Type = 'Regular'
            ];

            state.groupMemberChildRelationships = new List<String>();
            for (Schema.ChildRelationship relationship : Schema.SObjectType.Group.getChildRelationships()) {
                if (relationship.getChildSObject() == GroupMember.SObjectType &&
                    relationship.getField() == GroupMember.GroupId) {
                    state.groupMemberChildRelationships.add(relationship.getRelationshipName());
                }
            }
            state.groupMemberChildRelationships.sort();

            Set<String> selectedValues = new Set<String>{
                'Regular', 'Role', 'RoleAndSubordinatesInternal', 'RoleAndSubordinates', 'Queue'
            };
            state.selectedTypePicklistValues = new List<String>();
            for (Schema.PicklistEntry value : Schema.SObjectType.Group.fields.Type.getPicklistValues()) {
                if (selectedValues.contains(value.getValue()) && value.isActive()) {
                    state.selectedTypePicklistValues.add(value.getValue());
                }
            }
        state.selectedTypePicklistValues.sort();
        return state;
    }

    public static Map<String, Object> snapshot(State state) {
        return new Map<String, Object>{
            'roleQueryAccess' => state.roleGroupCount,
            'roleGroupTypes' => state.roleGroupTypes,
            'internalRoleGroups' => state.internalRoleGroupCount,
            'groupMetricsCardinality' => new Map<String, Object>{
                'childQuery' => state.memberQueryCount,
                'filteredQuery' => state.filteredMemberCount
            },
            'childRelationshipMetadataFilter' => new Map<String, Object>{
                'groupMemberRelationships' => state.groupMemberChildRelationships,
                'selectedTypePicklistValues' => state.selectedTypePicklistValues
            }
        };
    }
}
