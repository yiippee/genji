-- test: empty table
CREATE TABLE test;
ALTER TABLE test ADD FIELD a DEFAULT 10;
SELECT name, sql FROM __genji_catalog WHERE type = "table" AND name = "test";
/* result:
{
  "name": "test",
  "sql": "CREATE TABLE test (a DEFAULT 10)"
}
*/

-- test: non empty fields
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (2), (3.14);
ALTER TABLE test ADD FIELD a DEFAULT 10;
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

-- test: with NULL fields
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (2), (3);
ALTER TABLE test ADD FIELD b DEFAULT 10;
SELECT * FROM test;
/* result:
{
  "a": 1.0,
  "b": 10.0
},
{
  "a": 2.0,
  "b": 10.0
},
{
  "a": 3.14,
  "b": 10.0
},
*/
