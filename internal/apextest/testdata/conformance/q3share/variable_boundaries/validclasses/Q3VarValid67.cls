@IsTest private class Q3VarValid67 { @IsTest static void observed() {
Schema.SObjectField token = Schema.AccountShare.RowCause; System.assertEquals('RowCause', token.getDescribe().getName());

} }
