package sosl_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sosl"
)

func TestParseSOSLUpdateViewStatLocalClause(t *testing.T) {
	query, err := sosl.Parse("FIND {GladeClauseProbe} RETURNING KnowledgeArticleVersion(Id) UPDATE VIEWSTAT")
	if err != nil {
		t.Fatalf("parse local Knowledge UPDATE VIEWSTAT clause: %v", err)
	}
	if !query.UpdateViewStat {
		t.Fatal("UPDATE VIEWSTAT flag = false, want true")
	}
	if len(query.Returning) != 1 || query.Returning[0].Object != "KnowledgeArticleVersion" {
		t.Fatalf("RETURNING projection = %#v, want one KnowledgeArticleVersion result", query.Returning)
	}
	if len(query.Returning[0].Fields) != 1 || query.Returning[0].Fields[0].Field != "Id" {
		t.Fatalf("RETURNING fields = %#v, want only Id", query.Returning[0].Fields)
	}
}

func TestParseSOSLUpdateViewStatDoesNotConsumeOtherHostedClauses(t *testing.T) {
	// R185/R186 parse TRACKING and comma composition before rejecting their
	// non-Knowledge target. R168-R173 report unavailable KnowledgeArticleVersion;
	// Knowledge acceptance/effects remain excluded behind the hosted boundary.
	query, err := sosl.Parse("FIND {GladeClauseProbe} RETURNING KnowledgeArticleVersion(Id) UPDATE TRACKING")
	if err != nil || !query.UpdateTracking {
		t.Fatalf("TRACKING syntax: %#v %v", query, err)
	}
	for _, tc := range []struct {
		name string
		text string
		want string
	}{
		{
			name: "using scope remains unsupported",
			text: "FIND {GladeClauseProbe} RETURNING KnowledgeArticleVersion(Id) USING SCOPE Mine",
			want: "SOSL USING SCOPE hosted search service",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sosl.Parse(tc.text)
			var unsupported *sosl.UnsupportedFeatureError
			if !errors.As(err, &unsupported) || unsupported.Message != tc.want {
				t.Fatalf("parse error = %#v, want UnsupportedFeatureError %q", err, tc.want)
			}
		})
	}

	// R178/R179: UPDATE is terminal; any subsequent UPDATE is unexpected.
	_, err = sosl.Parse("FIND {GladeClauseProbe} RETURNING KnowledgeArticleVersion(Id) UPDATE VIEWSTAT UPDATE VIEWSTAT")
	if err == nil || !strings.Contains(err.Error(), "unexpected token: UPDATE") {
		t.Fatalf("duplicate clause error = %v, want native UPDATE rejection", err)
	}
}
