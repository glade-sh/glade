package vm

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestJSONParserStrictHeapIdentity(t *testing.T) {
	source := `JSONParser p=JSON.createParser('{"items":[1,2,3],"ok":true}');
 JSONParser alias=p;
 p.nextToken(); p.nextToken(); p.clearCurrentToken();
 p.nextToken(); p.skipChildren();
 while (p.nextToken()!=null) { p.getText(); }
 System.assertEquals(null,alias.getCurrentToken());`
	var transcript string
	for _, cap := range []int{0, 500, 1000, 2000} {
		for _, off := range []bool{false, true} {
			result := runStrictHeapCase(t, source, cap, nil, off)
			transcript += fmt.Sprintf("%d %t %q %d %d %v %v\n", cap, off, result.err, result.heap, result.fullWalk, result.violations, result.scalars)
		}
	}
	got := fmt.Sprintf("%x", sha256.Sum256([]byte(transcript)))
	if got != "5553aa576f9b7de6771865f175209df601586acebc4a5c4bfb648c8682d0f52d" {
		t.Fatalf("heap digest %s; transcript %s", got, transcript)
	}
}
