package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// The API 68 runtime surfaces fixed from apextest api68_native (anonymous
// route there) also hold inside an @IsTest method at source APIs 67.0 and 68.0:
// Limits Data Cloud getters (limits LIMITS-DC/FLEX), the Crypto key gate
// (crypto-ec, crypto-jwe), the JweToken container (JWE-CONTAINER), the
// QueryOptions builder (QO-BUILDER) and SET OPTIONS (QO-NULL, QO-SETOPTIONS).
func TestAPI68NativeSurfacesIsTestRoute(t *testing.T) {
	const body = `@IsTest private class Api68Surfaces {
    @IsTest static void dataCloudLimits() {
        System.assertEquals('0/50/0/50000000/0/2147483647', Limits.getDataCloudCursors() + '/' + Limits.getLimitDataCloudCursors() + '/' + Limits.getDataCloudCursorRows() + '/' + Limits.getLimitDataCloudCursorRows() + '/' + Limits.getDataCloudFlexCredits() + '/' + Limits.getLimitDataCloudFlexCredits());
    }
    @IsTest static void cryptoGate() {
        String got;
        try { Crypto.generateKeyPair('ES256'); } catch (SecurityException e) { got = e.getMessage(); }
        System.assertEquals('This crypto method is not available. Contact your Salesforce administrator.', got);
    }
    @IsTest static void jweToken() {
        Blob b = Blob.valueOf('ab');
        System.JweToken t = new System.JweToken('ECDH-ES+A128KW', 'A128GCM', b, b, b, b, b, Blob.valueOf(''));
        System.assertEquals('System.JweToken[alg=ECDH-ES+A128KW, enc=A128GCM, epkLen=2, encKeyLen=2, ivLen=2, ciphertextLen=2, tagLen=2, aadLen=0]', t.toString());
        System.assert(t.getIv() == b);
    }
    @IsTest static void queryOptions() {
        Database.QueryOptionsBuilder b = Database.QueryOptions.builder().withHonorEmptyStrings(false);
        Database.QueryOptions before = b.build();
        b.withHonorEmptyStrings(true);
        System.assertEquals(false, before.getHonorEmptyStrings());
        System.assertEquals(true, b.build().getHonorEmptyStrings());
        String got;
        try { Database.QueryOptions.builder().withDataspace(''); } catch (IllegalArgumentException e) { got = e.getMessage(); }
        System.assertEquals('dataspace cannot be null or empty', got);
    }
    @IsTest static void setOptions() {
        insert new Account(Name = 'S68');
        Database.QueryOptions opt = Database.QueryOptions.builder().withDataspace('default').build();
        System.assertEquals(1, [SELECT Id FROM Account LIMIT 1 SET OPTIONS :opt].size());
        Database.QueryOptions options = null;
        String got;
        try { Database.query('SELECT Id FROM Account LIMIT 0 SET OPTIONS :options'); } catch (QueryException e) { got = e.getMessage(); }
        System.assertEquals('SET OPTIONS bind value cannot be null', got);
    }
}`
	for _, api := range []string{"67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer","features":[]}`)
			path := filepath.Join(root, "force-app", "main", "default", "classes", "Api68Surfaces.cls")
			writeFile(t, path, body)
			writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 5 || summary.Passed != 5 {
				data, _ := json.Marshal(run)
				t.Fatalf("@IsTest route: %s", data)
			}
		})
	}
}
