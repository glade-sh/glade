@IsTest public class SecurityCapture { @IsTest static void capture() { System.assert(false,'SECURITY_ROWS|'+JSON.serialize(SecurityModelProbe.rows())); } }
