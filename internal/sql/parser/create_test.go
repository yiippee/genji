package parser_test

import (
	"math"
	"testing"

	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/internal/database"
	"github.com/genjidb/genji/internal/expr"
	"github.com/genjidb/genji/internal/query/statement"
	"github.com/genjidb/genji/internal/sql/parser"
	"github.com/genjidb/genji/internal/testutil"
	"github.com/genjidb/genji/internal/testutil/assert"
	"github.com/genjidb/genji/types"
	"github.com/stretchr/testify/require"
)

func TestParserCreateTable(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		expected statement.Statement
		errored  bool
	}{
		{"Basic", "CREATE TABLE test", &statement.CreateTableStmt{Info: database.TableInfo{TableName: "test"}}, false},
		{"If not exists", "CREATE TABLE IF NOT EXISTS test", &statement.CreateTableStmt{Info: database.TableInfo{TableName: "test"}, IfNotExists: true}, false},
		{"Path only", "CREATE TABLE test(a)", nil, true},
		{"With primary key", "CREATE TABLE test(foo INTEGER PRIMARY KEY)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), Type: types.IntegerValue},
					},
					TableConstraints: []*database.TableConstraint{
						{Name: "test_pk", Paths: testutil.ParseDocumentPaths(t, "foo"), PrimaryKey: true},
					},
				},
			}, false},
		{"With primary key twice", "CREATE TABLE test(foo PRIMARY KEY PRIMARY KEY)", nil, true},
		{"With type", "CREATE TABLE test(foo INTEGER)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), Type: types.IntegerValue},
					},
				},
			}, false},
		{"With not null", "CREATE TABLE test(foo NOT NULL)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), IsNotNull: true},
					},
				},
			}, false},
		{"With default", "CREATE TABLE test(foo DEFAULT \"10\")",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), DefaultValue: expr.Constraint(expr.LiteralValue{Value: types.NewTextValue("10")})},
					},
				},
			}, false},
		{"With default", "CREATE TABLE test(foo DEFAULT (\"10\"))",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), DefaultValue: expr.Constraint(expr.LiteralValue{Value: types.NewTextValue("10")})},
					},
				},
			}, false},
		{"With default twice", "CREATE TABLE test(foo DEFAULT 10 DEFAULT 10)", nil, true},
		{"With default and no parentheses", "CREATE TABLE test(foo DEFAULT (10)", nil, true},
		{"With forbidden tokens", "CREATE TABLE test(foo DEFAULT a)", nil, true},
		{"With forbidden tokens", "CREATE TABLE test(foo DEFAULT 1 AND 2)", nil, true},
		{"With unique", "CREATE TABLE test(foo.bar[0].baz UNIQUE)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					TableConstraints: []*database.TableConstraint{
						{Name: "test_foo.bar[0].baz_unique", Paths: testutil.ParseDocumentPaths(t, "foo.bar[0].baz"), Unique: true},
					},
				},
			}, false},

		{"With not null twice", "CREATE TABLE test(foo NOT NULL NOT NULL)", nil, true},
		{"With unique twice", "CREATE TABLE test(foo UNIQUE UNIQUE)", &statement.CreateTableStmt{
			Info: database.TableInfo{
				TableName: "test",
				TableConstraints: []*database.TableConstraint{
					{Name: "test_foo_unique", Paths: testutil.ParseDocumentPaths(t, "foo"), Unique: true},
				},
			},
		}, false},
		{"With check", "CREATE TABLE test(a CHECK(a > 10), b int CHECK(a > 20) CHECK(b > 10), CHECK(a > 30))",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "b")), Type: types.IntegerValue},
					},
					TableConstraints: []*database.TableConstraint{
						{Name: "test_check", Check: expr.Constraint(testutil.ParseExpr(t, "a > 10"))},
						{Name: "test_check1", Check: expr.Constraint(testutil.ParseExpr(t, "a > 20"))},
						{Name: "test_check2", Check: expr.Constraint(testutil.ParseExpr(t, "b > 10"))},
						{Name: "test_check3", Check: expr.Constraint(testutil.ParseExpr(t, "a > 30"))},
					},
				},
			},
			false},
		{"With type and not null", "CREATE TABLE test(foo INTEGER NOT NULL)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), Type: types.IntegerValue, IsNotNull: true},
					},
				},
			}, false},
		{"With not null and primary key", "CREATE TABLE test(foo INTEGER NOT NULL PRIMARY KEY)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), Type: types.IntegerValue, IsNotNull: true},
					},
					TableConstraints: []*database.TableConstraint{
						{Name: "test_pk", Paths: testutil.ParseDocumentPaths(t, "foo"), PrimaryKey: true},
					},
				},
			}, false},
		{"With primary key and not null", "CREATE TABLE test(foo INTEGER PRIMARY KEY NOT NULL)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), Type: types.IntegerValue, IsNotNull: true},
					},
					TableConstraints: []*database.TableConstraint{
						{Name: "test_pk", Paths: testutil.ParseDocumentPaths(t, "foo"), PrimaryKey: true},
					},
				},
			}, false},
		{"With multiple constraints", "CREATE TABLE test(foo INTEGER PRIMARY KEY, bar INTEGER NOT NULL, baz[4][1].bat TEXT UNIQUE)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), Type: types.IntegerValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "bar")), Type: types.IntegerValue, IsNotNull: true},
						{Path: document.Path(testutil.ParseDocumentPath(t, "baz[4][1].bat")), Type: types.TextValue},
					},
					TableConstraints: []*database.TableConstraint{
						{Name: "test_pk", Paths: testutil.ParseDocumentPaths(t, "foo"), PrimaryKey: true},
						{Name: "test_baz[4][1].bat_unique", Paths: testutil.ParseDocumentPaths(t, "baz[4][1].bat"), Unique: true},
					},
				},
			}, false},
		{"With table constraints / PK on defined field", "CREATE TABLE test(foo INTEGER, bar NOT NULL, PRIMARY KEY (foo))",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), Type: types.IntegerValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "bar")), IsNotNull: true},
					},
					TableConstraints: []*database.TableConstraint{
						{Name: "test_pk", Paths: testutil.ParseDocumentPaths(t, "foo"), PrimaryKey: true},
					},
				},
			}, false},
		{"With table constraints / PK on undefined field", "CREATE TABLE test(foo INTEGER, PRIMARY KEY (bar))",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), Type: types.IntegerValue},
					},
					TableConstraints: []*database.TableConstraint{
						{Name: "test_pk", Paths: testutil.ParseDocumentPaths(t, "bar"), PrimaryKey: true},
					},
				},
			}, false},
		{"With table constraints / field constraint after table constraint", "CREATE TABLE test(PRIMARY KEY (bar), foo INTEGER)", nil, true},
		{"With table constraints / duplicate pk", "CREATE TABLE test(foo INTEGER PRIMARY KEY, PRIMARY KEY (bar))", nil, true},
		{"With table constraints / duplicate pk on same path", "CREATE TABLE test(foo INTEGER PRIMARY KEY, PRIMARY KEY (foo))", nil, true},
		{"With table constraints / UNIQUE on defined field", "CREATE TABLE test(foo INTEGER, bar NOT NULL, UNIQUE (foo))",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), Type: types.IntegerValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "bar")), IsNotNull: true},
					},
					TableConstraints: []*database.TableConstraint{
						{Name: "test_foo_unique", Paths: testutil.ParseDocumentPaths(t, "foo"), Unique: true},
					},
				},
			}, false},
		{"With table constraints / UNIQUE on undefined field", "CREATE TABLE test(foo INTEGER, UNIQUE (bar))",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), Type: types.IntegerValue},
					},
					TableConstraints: []*database.TableConstraint{
						{Name: "test_bar_unique", Paths: testutil.ParseDocumentPaths(t, "bar"), Unique: true},
					},
				},
			}, false},
		{"With table constraints / UNIQUE twice", "CREATE TABLE test(foo INTEGER UNIQUE, UNIQUE (foo))",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "foo")), Type: types.IntegerValue},
					},
					TableConstraints: []*database.TableConstraint{
						{Name: "test_foo_unique", Paths: testutil.ParseDocumentPaths(t, "foo"), Unique: true},
					},
				},
			}, false},
		{"With table constraints / duplicate pk on same path", "CREATE TABLE test(foo INTEGER PRIMARY KEY, PRIMARY KEY (foo))", nil, true},
		{"With multiple primary keys", "CREATE TABLE test(foo PRIMARY KEY, bar PRIMARY KEY)", nil, true},
		{"With all supported fixed size data types",
			"CREATE TABLE test(d double, b bool)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "d")), Type: types.DoubleValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "b")), Type: types.BoolValue},
					},
				},
			}, false},
		{"With all supported variable size data types",
			"CREATE TABLE test(i integer, b blob, byt bytes, t text, a array, d document)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "i")), Type: types.IntegerValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "b")), Type: types.BlobValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "byt")), Type: types.BlobValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "t")), Type: types.TextValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "a")), Type: types.ArrayValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "d")), Type: types.DocumentValue},
					},
				},
			}, false},
		{"With integer aliases types",
			"CREATE TABLE test(i int, ii int2, ei int8, m mediumint, s smallint, b bigint, t tinyint)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "i")), Type: types.IntegerValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "ii")), Type: types.IntegerValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "ei")), Type: types.IntegerValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "m")), Type: types.IntegerValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "s")), Type: types.IntegerValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "b")), Type: types.IntegerValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "t")), Type: types.IntegerValue},
					},
				},
			}, false},
		{"With double aliases types",
			"CREATE TABLE test(dp DOUBLE PRECISION, r real, d double)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "dp")), Type: types.DoubleValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "r")), Type: types.DoubleValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "d")), Type: types.DoubleValue},
					},
				},
			}, false},
		{"With text aliases types",
			"CREATE TABLE test(v VARCHAR(255), c CHARACTER(64), t TEXT)",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "v")), Type: types.TextValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "c")), Type: types.TextValue},
						{Path: document.Path(testutil.ParseDocumentPath(t, "t")), Type: types.TextValue},
					},
				},
			}, false},
		{"With errored text aliases types",
			"CREATE TABLE test(v VARCHAR(1 IN [1, 2, 3] AND foo > 4) )",
			&statement.CreateTableStmt{
				Info: database.TableInfo{
					TableName: "test",
					FieldConstraints: []*database.FieldConstraint{
						{Path: document.Path(testutil.ParseDocumentPath(t, "v")), Type: types.TextValue},
					},
				},
			}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q, err := parser.ParseQuery(test.s)
			if test.errored {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			require.Len(t, q.Statements, 1)
			require.EqualValues(t, test.expected, q.Statements[0])
		})
	}
}

func TestParserCreateIndex(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		expected statement.Statement
		errored  bool
	}{
		{"Basic", "CREATE INDEX idx ON test (foo)", &statement.CreateIndexStmt{
			Info: database.IndexInfo{
				IndexName: "idx", TableName: "test", Paths: []document.Path{document.Path(testutil.ParseDocumentPath(t, "foo"))},
			}}, false},
		{"If not exists", "CREATE INDEX IF NOT EXISTS idx ON test (foo.bar[1])", &statement.CreateIndexStmt{
			Info: database.IndexInfo{
				IndexName: "idx", TableName: "test", Paths: []document.Path{document.Path(testutil.ParseDocumentPath(t, "foo.bar[1]"))},
			}, IfNotExists: true}, false},
		{"Unique", "CREATE UNIQUE INDEX IF NOT EXISTS idx ON test (foo[3].baz)", &statement.CreateIndexStmt{
			Info: database.IndexInfo{
				IndexName: "idx", TableName: "test", Paths: []document.Path{document.Path(testutil.ParseDocumentPath(t, "foo[3].baz"))}, Unique: true,
			}, IfNotExists: true}, false},
		{"No name", "CREATE UNIQUE INDEX ON test (foo[3].baz)", &statement.CreateIndexStmt{
			Info: database.IndexInfo{TableName: "test", Paths: []document.Path{document.Path(testutil.ParseDocumentPath(t, "foo[3].baz"))}, Unique: true}}, false},
		{"No name with IF NOT EXISTS", "CREATE UNIQUE INDEX IF NOT EXISTS ON test (foo[3].baz)", nil, true},
		{"More than 1 path", "CREATE INDEX idx ON test (foo, bar)",
			&statement.CreateIndexStmt{
				Info: database.IndexInfo{
					IndexName: "idx",
					TableName: "test",
					Paths: []document.Path{
						document.Path(testutil.ParseDocumentPath(t, "foo")),
						document.Path(testutil.ParseDocumentPath(t, "bar")),
					},
				},
			},
			false},
		{"No fields", "CREATE INDEX idx ON test", nil, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q, err := parser.ParseQuery(test.s)
			if test.errored {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			require.Len(t, q.Statements, 1)
			require.EqualValues(t, test.expected, q.Statements[0])
		})
	}
}

func TestParserCreateSequence(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		expected statement.Statement
		errored  bool
	}{
		{"Basic", "CREATE SEQUENCE seq", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 1, Max: math.MaxInt64, Start: 1, Cache: 1},
		}, false},
		{"If not exists", "CREATE SEQUENCE IF NOT EXISTS seq", &statement.CreateSequenceStmt{
			Info:        database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 1, Max: math.MaxInt64, Start: 1, Cache: 1},
			IfNotExists: true,
		}, false},
		{"AS integer", "CREATE SEQUENCE seq AS TINYINT", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 1, Max: math.MaxInt64, Start: 1, Cache: 1},
		}, false},
		{"AS double", "CREATE SEQUENCE seq AS DOUBLE", nil, true},
		{"INCREMENT", "CREATE SEQUENCE seq INCREMENT 10", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 10, Min: 1, Max: math.MaxInt64, Start: 1, Cache: 1},
		}, false},
		{"INCREMENT BY", "CREATE SEQUENCE seq INCREMENT BY 10", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 10, Min: 1, Max: math.MaxInt64, Start: 1, Cache: 1},
		}, false},
		{"INCREMENT BY 0", "CREATE SEQUENCE seq INCREMENT BY 0", nil, true},
		{"NO MINVALUE", "CREATE SEQUENCE seq NO MINVALUE", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 1, Max: math.MaxInt64, Start: 1, Cache: 1},
		}, false},
		{"NO MAXVALUE", "CREATE SEQUENCE seq NO MAXVALUE", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 1, Max: math.MaxInt64, Start: 1, Cache: 1},
		}, false},
		{"NO CYCLE", "CREATE SEQUENCE seq NO CYCLE", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 1, Max: math.MaxInt64, Start: 1, Cache: 1},
		}, false},
		{"NO SUGAR", "CREATE SEQUENCE seq NO SUGAR", nil, true},
		{"MINVALUE 10", "CREATE SEQUENCE seq MINVALUE 10", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 10, Max: math.MaxInt64, Start: 10, Cache: 1},
		}, false},
		{"MINVALUE 'hello'", "CREATE SEQUENCE seq MINVALUE 'hello'", nil, true},
		{"MAXVALUE 10", "CREATE SEQUENCE seq MAXVALUE 10", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 1, Max: 10, Start: 1, Cache: 1},
		}, false},
		{"MAXVALUE 'hello'", "CREATE SEQUENCE seq MAXVALUE 'hello'", nil, true},
		{"START WITH 10", "CREATE SEQUENCE seq START WITH 10", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 1, Max: math.MaxInt64, Start: 10, Cache: 1},
		}, false},
		{"START WITH 'hello'", "CREATE SEQUENCE seq START WITH 'hello'", nil, true},
		{"START 10", "CREATE SEQUENCE seq START 10", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 1, Max: math.MaxInt64, Start: 10, Cache: 1},
		}, false},
		{"CACHE 10", "CREATE SEQUENCE seq CACHE 10", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 1, Max: math.MaxInt64, Start: 1, Cache: 10},
		}, false},
		{"CACHE 'hello'", "CREATE SEQUENCE seq CACHE 'hello'", nil, true},
		{"CACHE -10", "CREATE SEQUENCE seq CACHE -10", nil, true},
		{"CYCLE", "CREATE SEQUENCE seq CYCLE", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: 1, Min: 1, Max: math.MaxInt64, Start: 1, Cache: 1, Cycle: true},
		}, false},
		{"Order 1", `
			CREATE SEQUENCE IF NOT EXISTS seq
			AS INTEGER
			INCREMENT BY 2
			NO MINVALUE
			MAXVALUE 10
			START WITH 5
			CACHE 5
			CYCLE
		`, &statement.CreateSequenceStmt{
			IfNotExists: true,
			Info: database.SequenceInfo{
				Name:        "seq",
				IncrementBy: 2,
				Min:         1,
				Max:         10,
				Start:       5,
				Cache:       5,
				Cycle:       true,
			},
		}, false},
		{"Order 2", `
			CREATE SEQUENCE IF NOT EXISTS seq
			CYCLE
			MAXVALUE 10
			INCREMENT BY 2
			START WITH 5
			AS INTEGER
			NO MINVALUE
			CACHE 5
		`, &statement.CreateSequenceStmt{
			IfNotExists: true,
			Info: database.SequenceInfo{
				Name:        "seq",
				IncrementBy: 2,
				Min:         1,
				Max:         10,
				Start:       5,
				Cache:       5,
				Cycle:       true,
			},
		}, false},
		{"NO MINVALUE with MINVALUE 10", "CREATE SEQUENCE seq NO MINVALUE MINVALUE 10", nil, true},
		{"NO MAXVALUE with MAXVALUE 10", "CREATE SEQUENCE seq NO MAXVALUE MAXVALUE 10", nil, true},
		{"NO CYCLE with CYCLE", "CREATE SEQUENCE seq NO MAXVALUE MAXVALUE 10", nil, true},
		{"duplicate AS INT", "CREATE SEQUENCE seq AS INT AS INT", nil, true},
		{"duplicate INCREMENT BY", "CREATE SEQUENCE seq INCREMENT BY 10 INCREMENT BY 10", nil, true},
		{"duplicate NO MINVALUE", "CREATE SEQUENCE seq NO MINVALUE NO MINVALUE", nil, true},
		{"duplicate NO MAXVALUE", "CREATE SEQUENCE seq NO MAXVALUE NO MAXVALUE", nil, true},
		{"duplicate NO CYCLE", "CREATE SEQUENCE seq NO CYCLE NO CYCLE", nil, true},
		{"duplicate MINVALUE", "CREATE SEQUENCE seq MINVALUE 10 MINVALUE 10", nil, true},
		{"duplicate MAXVALUE", "CREATE SEQUENCE seq MAXVALUE 10 MAXVALUE 10", nil, true},
		{"duplicate START WITH", "CREATE SEQUENCE seq START WITH 10 START WITH 10", nil, true},
		{"duplicate CACHE", "CREATE SEQUENCE seq CACHE 10 CACHE 10", nil, true},
		{"duplicate CYCLE", "CREATE SEQUENCE seq CYCLE CYCLE", nil, true},
		{"BAD MINVALUE MAXVALUE", "CREATE SEQUENCE seq MINVALUE 10 MAXVALUE 5", nil, true},
		{"BAD START", "CREATE SEQUENCE seq MINVALUE 5 MAXVALUE 10 START 100", nil, true},
		{"BAD START", "CREATE SEQUENCE seq MINVALUE 5 MAXVALUE 10 START -100", nil, true},
		{"MINVALUE 10 DESC", "CREATE SEQUENCE seq MINVALUE 10 MAXVALUE 100 INCREMENT BY -1", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: -1, Min: 10, Max: 100, Start: 100, Cache: 1},
		}, false},
		{"NO MINVALUE DESC", "CREATE SEQUENCE seq NO MINVALUE MAXVALUE 100 INCREMENT BY -1", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: -1, Min: math.MinInt64, Max: 100, Start: 100, Cache: 1},
		}, false},
		{"NO MAXVALUE DESC", "CREATE SEQUENCE seq NO MINVALUE NO MAXVALUE INCREMENT BY -1", &statement.CreateSequenceStmt{
			Info: database.SequenceInfo{Name: "seq", IncrementBy: -1, Min: math.MinInt64, Max: -1, Start: -1, Cache: 1},
		}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q, err := parser.ParseQuery(test.s)
			if test.errored {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			require.Len(t, q.Statements, 1)
			require.EqualValues(t, test.expected, q.Statements[0])
		})
	}
}
