package gladecli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunExecQueueableDuplicateSignatureWithProjectRuntime(t *testing.T) {
	tests := map[string]string{
		"qualified builder": `QueueableDuplicateSignature sig = QueueableDuplicateSignature.builder().addString('job').addInteger(42).addId('001000000000001AAA').build();
System.assert(sig.toString().contains('String:job'));
System.assert(sig.toString().contains('Integer:42'));
System.assert(sig.toString().contains('Id:001000000000001AAA'));`,
		"builder size": `QueueableDuplicateSignature.Builder builder = QueueableDuplicateSignature.builder();
System.assertEquals(0, builder.getSize());
System.assertEquals(10, builder.getMaxSize());
System.assertEquals(10, builder.getRemainingSize());
builder.addString('nightly');
builder.addInteger(7);
builder.addId('001000000000001AAA');
System.assertEquals(3, builder.getSize());
System.assertEquals(7, builder.getRemainingSize());
QueueableDuplicateSignature signature = builder.build();
System.assert(signature.toString().contains('String:nightly'));
System.assert(signature.toString().contains('Integer:7'));
System.assert(signature.toString().contains('Id:001000000000001AAA'));`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", ".", "--json", source}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("queueable duplicate signature execution failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if strings.Contains(stderr.String(), "NullPointerException") || strings.Contains(stderr.String(), "expected <") {
				t.Fatalf("queueable duplicate signature emitted runtime failure: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}
