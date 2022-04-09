-- test: empty table
CREATE TABLE test;
ALTER TABLE test ADD FIELD a UNIQUE;
SELECT name, sql FROM __genji_catalog WHERE type = "table" AND name = "test";
/* result:
{
  "name": "test",
  "sql": "CREATE TABLE test (a UNIQUE)"
}
*/

-- test: non empty, unique fields
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (2), (3.14);
ALTER TABLE test ADD FIELD a UNIQUE;
SELECT * FROM test;
/* result:
{
  "a": 1.0
},
{
  "a": 2.0
},
{
  "a": 3.14
}
*/

-- test: non empty, duplicate fields
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (2), (2), (3);
ALTER TABLE test ADD FIELD a UNIQUE;
-- error: TODO

-- test: empty field
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (2), (3);
ALTER TABLE test ADD FIELD b UNIQUE;
-- error: TODO

-- test: empty field, one document
CREATE TABLE test;
INSERT INTO test (a) VALUES (1);
ALTER TABLE test ADD FIELD b UNIQUE;
SELECT * FROM test;
/* result:
{
  "a": 1.0
}
*/