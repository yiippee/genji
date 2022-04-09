-- test: empty table
CREATE TABLE test;
ALTER TABLE test ADD FIELD a CHECK (a > 0);
SELECT name, sql FROM __genji_catalog WHERE type = "table" AND name = "test";
/* result:
{
  "name": "test",
  "sql": "CREATE TABLE test (CHECK (a > 0))"
}
*/

-- test: non empty, positive fields
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (2), (3.14);
ALTER TABLE test ADD FIELD a CHECK (a > 0);
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

-- test: non empty, negative fields
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (-2), (2), (3);
ALTER TABLE test ADD FIELD a CHECK (a > 0);
-- error: TODO

-- test: empty field
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (2), (3);
ALTER TABLE test ADD FIELD b CHECK (a > 0);
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
