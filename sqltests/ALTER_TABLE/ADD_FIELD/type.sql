-- test: same type
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (2), (3.14); -- doubles
ALTER TABLE test ADD FIELD a DOUBLE;
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

-- test: compatible type
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (2), (3); -- doubles
ALTER TABLE test ADD FIELD a INT;
SELECT * FROM test;
/* result:
{
  "a": 1
},
{
  "a": 2
},
{
  "a": 3
}
*/

-- test: incompatible type
CREATE TABLE test;
INSERT INTO test (a) VALUES (1), (2), (3.14); -- doubles
ALTER TABLE test ADD FIELD a INT;
-- error: TODO
