@IsTest
private class UnitRegressionProbe67 {
    @TestSetup
    static void prepareActor() {
        Profile standard = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        User actor = new User(
            Alias = 'unitreg',
            Email = 'unitreg@example.invalid',
            EmailEncodingKey = 'UTF-8',
            LastName = 'UnitRegression',
            LanguageLocaleKey = 'en_US',
            LocaleSidKey = 'en_US',
            ProfileId = standard.Id,
            TimeZoneSidKey = 'America/Los_Angeles',
            Username = 'unitreg.' + String.valueOf(Datetime.now().getTime()) + '@example.invalid'
        );
        insert actor;
        PermissionSet access = [SELECT Id FROM PermissionSet WHERE Name = 'WidgetEditAccess' LIMIT 1];
        insert new PermissionSetAssignment(AssigneeId = actor.Id, PermissionSetId = access.Id);
    }

    @IsTest
    static void observeDirectEditShare() {
        User actor = [SELECT Id FROM User WHERE Alias = 'unitreg' LIMIT 1];
        Widget__c row = new Widget__c(Name = 'before');
        insert row;
        insert new Widget__Share(
            ParentId = row.Id,
            UserOrGroupId = actor.Id,
            AccessLevel = 'Edit',
            RowCause = 'Manual'
        );

        Map<String, Object> actual = new Map<String, Object>();
        try {
            System.runAs(actor) {
                List<Widget__c> visible = [SELECT Id, Name FROM Widget__c WHERE Id = :row.Id];
                actual.put('visibleCount', visible.size());
                if (!visible.isEmpty()) {
                    visible[0].Name = 'changed';
                    update visible[0];
                    actual.put('updateSucceeded', true);
                }
            }
        } catch (Exception error) {
            actual.put('errorType', error.getTypeName());
            if (error instanceof DmlException) {
                actual.put('dmlStatus', String.valueOf(((DmlException)error).getDmlStatusCode(0)));
            }
        }
        actual.put('storedName', [SELECT Name FROM Widget__c WHERE Id = :row.Id].Name);
        report('direct_edit_share', actual);
    }

    @IsTest
    static void observeLooseJSONDate() {
        Map<String, Object> actual = new Map<String, Object>();
        try {
            Date value = (Date)JSON.deserialize('"2024-2-29 00:00:00"', Date.class);
            actual.put('value', String.valueOf(value));
        } catch (Exception error) {
            actual.put('errorType', error.getTypeName());
            actual.put('errorMessage', error.getMessage());
        }
        report('json_loose_date', actual);
    }

    @IsTest
    static void observeStaticFactory() {
        UnitRegressionGateway67 providerTarget = (UnitRegressionGateway67)Test.createStub(
            UnitRegressionGateway67.class, new GatewayProvider());
        UnitRegressionGateway67.mockInstance = providerTarget;
        report('stub_static_factory', new Map<String, Object>{'value' => UnitRegressionGateway67.newInstance().cancel()});
    }

    @IsTest
    static void observeStaticFactoryAsync() {
        UnitRegressionGateway67.mockInstance = (UnitRegressionGateway67)Test.createStub(
            UnitRegressionGateway67.class, new GatewayProvider());
        Test.startTest();
        System.enqueueJob(new GatewayJob());
        Test.stopTest();
        List<Account> rows = [SELECT Name FROM Account WHERE Name IN ('mock', 'real') ORDER BY Name];
        List<String> names = new List<String>();
        for (Account row : rows) names.add(row.Name);
        report('stub_static_factory_async', new Map<String, Object>{'names' => names});
    }

    @IsTest
    static void observeMapValuesListSObject() {
        DMLProvider provider = new DMLProvider();
        UnitRegressionDML67.mockInstance = (UnitRegressionDML67)Test.createStub(
            UnitRegressionDML67.class, provider);
        Account row = new Account(Name = 'map-value');
        insert row;
        Map<Id, Account> rows = new Map<Id, Account>{row.Id => row};
        UnitRegressionDML67.newInstance().updateRecords(rows.values());
        report('stub_map_values_list_sobject', new Map<String, Object>{
            'callbackCount' => provider.calls,
            'argumentType' => provider.argumentType
        });
    }

    @IsTest
    static void observePlatformTypeRejection() {
        GatewayProvider provider = new GatewayProvider();
        Map<String, Object> actual = new Map<String, Object>();
        try {
            Test.createStub(Account.class, provider);
            actual.put('errorType', null);
        } catch (Exception error) {
            actual.put('errorType', error.getTypeName());
            actual.put('errorMessage', error.getMessage());
        }
        actual.put('callbackCount', provider.calls);
        report('stub_platform_type_rejection', actual);
    }

    private static void report(String row, Object value) {
        System.assert(false, 'NATIVE|' + row + '|' + JSON.serialize(value));
    }

    public class GatewayProvider implements System.StubProvider {
        public Integer calls = 0;

        public Object handleMethodCall(
            Object stubbedObject,
            String stubbedMethodName,
            Type returnType,
            List<Type> listOfParamTypes,
            List<String> listOfParamNames,
            List<Object> listOfArgs
        ) {
            calls++;
            return 'mock';
        }
    }

    public class DMLProvider implements System.StubProvider {
        public Integer calls = 0;
        public String argumentType;

        public Object handleMethodCall(
            Object stubbedObject,
            String stubbedMethodName,
            Type returnType,
            List<Type> listOfParamTypes,
            List<String> listOfParamNames,
            List<Object> listOfArgs
        ) {
            calls++;
            argumentType = String.valueOf(listOfParamTypes[0]);
            return new List<Database.SaveResult>();
        }
    }

    public class GatewayJob implements Queueable {
        public void execute(QueueableContext context) {
            insert new Account(Name = UnitRegressionGateway67.newInstance().cancel());
        }
    }
}
