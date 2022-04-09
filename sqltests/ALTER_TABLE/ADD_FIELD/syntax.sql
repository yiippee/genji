-- test: ADD
CREATE TABLE test(a int);
ALTER TABLE test ADD b DOUBLE;
SELECT name, sql FROM __genji_catalog WHERE type = "table" AND name = "test";
/* result:
{
  "name": "test",
  "sql": "CREATE TABLE test (a INTEGER, b DOUBLE)"
}
*/

-- test: ADD FIELD
CREATE TABLE test(a int);
ALTER TABLE test ADD FIELD b DOUBLE;
SELECT name, sql FROM __genji_catalog WHERE type = "table" AND name = "test";
/* result:
{
  "name": "test",
  "sql": "CREATE TABLE test (a INTEGER, b DOUBLE)"
}
*/

-- test: ADD existing field
CREATE TABLE test(a int);
ALTER TABLE test ADD a INT;
-- error: TODO

-- test: ADD IF NOT EXISTS / existing field
CREATE TABLE test(a int);
ALTER TABLE test ADD IF NOT EXISTS a INT;
SELECT name, sql FROM __genji_catalog WHERE type = "table" AND name = "test";
/* result:
{
  "name": "test",
  "sql": "CREATE TABLE test (a INTEGER)"
}
*/

-- test: ADD IF NOT EXISTS / non-existing field
CREATE TABLE test(a int);
ALTER TABLE test ADD IF NOT EXISTS b DOUBLE;
SELECT name, sql FROM __genji_catalog WHERE type = "table" AND name = "test";
/* result:
{
  "name": "test",
  "sql": "CREATE TABLE test (a INTEGER, b DOUBLE)"
}
*/
