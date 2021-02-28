package parser

import (
	"testing"

	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/expr"
	"github.com/genjidb/genji/planner"
	"github.com/genjidb/genji/stream"
	"github.com/stretchr/testify/require"
)

func TestParserUpdate(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		expected *stream.Stream
		errored  bool
	}{
		{"SET/No cond", "UPDATE test SET a = 1",
			stream.New(stream.SeqScan("test")).
				Pipe(stream.Set(document.Path(parsePath(t, "a")), expr.IntegerValue(1))).
				Pipe(stream.TableReplace("test")),
			false,
		},
		{"SET/With cond", "UPDATE test SET a = 1, b = 2 WHERE age = 10",
			stream.New(stream.SeqScan("test")).
				Pipe(stream.Filter(MustParseExpr("age = 10"))).
				Pipe(stream.Set(document.Path(parsePath(t, "a")), expr.IntegerValue(1))).
				Pipe(stream.Set(document.Path(parsePath(t, "b")), MustParseExpr("2"))).
				Pipe(stream.TableReplace("test")),
			false,
		},
		{"SET/No cond path with backquotes", "UPDATE test SET `   some \"path\" ` = 1",
			stream.New(stream.SeqScan("test")).
				Pipe(stream.Set(document.Path(parsePath(t, "`   some \"path\" `")), expr.IntegerValue(1))).
				Pipe(stream.TableReplace("test")),
			false,
		},
		{"SET/No cond nested path", "UPDATE test SET a.b = 1",
			stream.New(stream.SeqScan("test")).
				Pipe(stream.Set(document.Path(parsePath(t, "a.b")), expr.IntegerValue(1))).
				Pipe(stream.TableReplace("test")),
			false,
		},
		{"UNSET/No cond", "UPDATE test UNSET a",
			stream.New(stream.SeqScan("test")).
				Pipe(stream.Unset("a")).
				Pipe(stream.TableReplace("test")),
			false,
		},
		{"UNSET/With cond", "UPDATE test UNSET a, b WHERE age = 10",
			stream.New(stream.SeqScan("test")).
				Pipe(stream.Filter(MustParseExpr("age = 10"))).
				Pipe(stream.Unset("a")).
				Pipe(stream.Unset("b")).
				Pipe(stream.TableReplace("test")),
			false,
		},
		{"Trailing comma", "UPDATE test SET a = 1, WHERE age = 10", nil, true},
		{"No SET", "UPDATE test WHERE age = 10", nil, true},
		{"No pair", "UPDATE test SET WHERE age = 10", nil, true},
		{"query.Field only", "UPDATE test SET a WHERE age = 10", nil, true},
		{"No value", "UPDATE test SET a = WHERE age = 10", nil, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q, err := ParseQuery(test.s)
			if test.errored {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, q.Statements, 1)
			stmt := q.Statements[0].(*planner.Statement)
			require.False(t, stmt.ReadOnly)
			require.EqualValues(t, test.expected, stmt.Stream)
		})
	}
}
