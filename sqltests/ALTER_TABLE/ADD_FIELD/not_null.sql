-- test: empty table
CREATE TABLE test;
ALTER TABLE test ADD FIELD a NOT NULL;
SELECT name, sql FROM __genji_catalog WHERE type = "table" AND name = "test";
/* result:
{
  "name": "test",
  "sql": "CREATE TABLE test (a NOT NULL)"
}
*/

-- test: non empty fields
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (2), (3.14);
ALTER TABLE test ADD FIELD a NOT NULL;
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
ALTER TABLE test ADD FIELD b NOT NULL;
-- error: TODO
