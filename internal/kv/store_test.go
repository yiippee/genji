package kv_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/genjidb/genji"
	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/internal/kv"
	"github.com/genjidb/genji/internal/testutil"
	"github.com/genjidb/genji/internal/testutil/assert"
	"github.com/genjidb/genji/types"
	"github.com/stretchr/testify/require"
)

func getValue(t *testing.T, st *kv.Namespace, key []byte) []byte {
	v, err := st.Get([]byte(key))
	assert.NoError(t, err)
	return v
}

func TestReadOnly(t *testing.T) {
	pdb := testutil.NewPebble(t)

	t.Run("Read-Only write attempts", func(t *testing.T) {
		sro := kv.NewSession(pdb, true)

		// fetch the store and the index
		st := sro.GetNamespace(10)

		tests := []struct {
			name string
			fn   func(*error)
		}{
			{"StorePut", func(err *error) { *err = st.Put([]byte("id"), nil) }},
			{"StoreDelete", func(err *error) { *err = st.Delete([]byte("id")) }},
			{"StoreTruncate", func(err *error) { *err = st.Truncate() }},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				var err error
				test.fn(&err)

				assert.Error(t, err)
			})
		}
	})
}

func TestGetNamespace(t *testing.T) {
	t.Run("Should return the right store", func(t *testing.T) {
		pdb := testutil.NewPebble(t)

		s := kv.NewSession(pdb, false)

		// fetch first store
		sta := s.GetNamespace(10)

		// fetch second store
		stb := s.GetNamespace(20)

		// insert data in first store
		err := sta.Put([]byte("foo"), []byte("FOO"))
		assert.NoError(t, err)

		// use sta to fetch data and verify if it's present
		v := getValue(t, sta, []byte("foo"))
		require.Equal(t, v, []byte("FOO"))

		// use stb to fetch data and verify it's not present
		_, err = stb.Get([]byte("foo"))
		assert.ErrorIs(t, err, kv.ErrKeyNotFound)
	})
}

func storeBuilder(t testing.TB) *kv.Namespace {
	pdb := testutil.NewPebble(t)

	s := kv.NewSession(pdb, false)

	st := s.GetNamespace(10)
	return st
}

func TestStorePut(t *testing.T) {
	t.Run("Should insert data", func(t *testing.T) {
		st := storeBuilder(t)

		err := st.Put([]byte("foo"), []byte("FOO"))
		assert.NoError(t, err)

		v := getValue(t, st, []byte("foo"))
		require.Equal(t, []byte("FOO"), v)
	})

	t.Run("Should replace existing key", func(t *testing.T) {
		st := storeBuilder(t)

		err := st.Put([]byte("foo"), []byte("FOO"))
		assert.NoError(t, err)

		err = st.Put([]byte("foo"), []byte("BAR"))
		assert.NoError(t, err)

		v := getValue(t, st, []byte("foo"))
		require.Equal(t, []byte("BAR"), v)
	})

	t.Run("Should fail when key is nil or empty", func(t *testing.T) {
		st := storeBuilder(t)

		err := st.Put(nil, []byte("FOO"))
		assert.Error(t, err)

		err = st.Put([]byte(""), []byte("BAR"))
		assert.Error(t, err)
	})

	t.Run("Should fail when value is nil or empty", func(t *testing.T) {
		st := storeBuilder(t)

		err := st.Put([]byte("foo"), nil)
		assert.Error(t, err)

		err = st.Put([]byte("foo"), []byte(""))
		assert.Error(t, err)
	})
}

// TestStoreGet verifies Get behaviour.
func TestStoreGet(t *testing.T) {
	t.Run("Should fail if not found", func(t *testing.T) {
		st := storeBuilder(t)

		r, err := st.Get([]byte("id"))
		assert.ErrorIs(t, err, kv.ErrKeyNotFound)
		require.Nil(t, r)
	})

	t.Run("Should return the right key", func(t *testing.T) {
		st := storeBuilder(t)

		err := st.Put([]byte("foo"), []byte("FOO"))
		assert.NoError(t, err)
		err = st.Put([]byte("bar"), []byte("BAR"))
		assert.NoError(t, err)

		v := getValue(t, st, []byte("foo"))
		require.Equal(t, []byte("FOO"), v)

		v = getValue(t, st, []byte("bar"))
		require.Equal(t, []byte("BAR"), v)
	})
}

// TestStoreDelete verifies Delete behaviour.
func TestStoreDelete(t *testing.T) {
	t.Run("Should fail if not found", func(t *testing.T) {
		st := storeBuilder(t)

		err := st.Delete([]byte("id"))
		assert.ErrorIs(t, err, kv.ErrKeyNotFound)
	})

	t.Run("Should delete the right document", func(t *testing.T) {
		st := storeBuilder(t)

		err := st.Put([]byte("foo"), []byte("FOO"))
		assert.NoError(t, err)
		err = st.Put([]byte("bar"), []byte("BAR"))
		assert.NoError(t, err)

		v := getValue(t, st, []byte("foo"))
		require.Equal(t, []byte("FOO"), v)

		// delete the key
		err = st.Delete([]byte("bar"))
		assert.NoError(t, err)

		// try again, should fail
		err = st.Delete([]byte("bar"))
		assert.ErrorIs(t, err, kv.ErrKeyNotFound)

		// make sure it didn't also delete the other one
		v = getValue(t, st, []byte("foo"))
		require.Equal(t, []byte("FOO"), v)

		// the deleted key must not appear on iteration
		it := st.Iterator(nil)
		defer it.Close()
		i := 0
		for it.First(); it.Valid(); it.Next() {
			require.Equal(t, []byte("foo"), it.Key()[4:])
			i++
		}
		require.Equal(t, 1, i)
	})
}

func TestStoreTruncate(t *testing.T) {
	t.Run("Should succeed if store is empty", func(t *testing.T) {
		st := storeBuilder(t)

		err := st.Truncate()
		assert.NoError(t, err)
	})

	t.Run("Should truncate the store", func(t *testing.T) {
		st := storeBuilder(t)

		err := st.Put([]byte("foo"), []byte("FOO"))
		assert.NoError(t, err)
		err = st.Put([]byte("bar"), []byte("BAR"))
		assert.NoError(t, err)

		err = st.Truncate()
		assert.NoError(t, err)

		it := st.Iterator(nil)
		defer it.Close()
		it.First()
		assert.NoError(t, it.Error())
		require.False(t, it.Valid())
	})
}

// TestQueries test simple queries against the kv.
func TestQueries(t *testing.T) {
	t.Run("SELECT", func(t *testing.T) {
		dir := testutil.TempDir(t)

		db, err := genji.Open(filepath.Join(dir, "pebble"))
		assert.NoError(t, err)

		d, err := db.QueryDocument(`
			CREATE TABLE test;
			INSERT INTO test (a) VALUES (1), (2), (3), (4);
			SELECT COUNT(*) FROM test;
		`)
		assert.NoError(t, err)
		var count int
		err = document.Scan(d, &count)
		assert.NoError(t, err)
		require.Equal(t, 4, count)

		t.Run("ORDER BY", func(t *testing.T) {
			st, err := db.Query("SELECT * FROM test ORDER BY a DESC")
			assert.NoError(t, err)
			defer st.Close()

			var i int
			err = st.Iterate(func(d types.Document) error {
				var a int
				err := document.Scan(d, &a)
				assert.NoError(t, err)
				require.Equal(t, 4-i, a)
				i++
				return nil
			})
			assert.NoError(t, err)
		})
	})

	t.Run("INSERT", func(t *testing.T) {
		dir := testutil.TempDir(t)

		db, err := genji.Open(filepath.Join(dir, "pebble"))
		assert.NoError(t, err)

		err = db.Exec(`
			CREATE TABLE test;
			INSERT INTO test (a) VALUES (1), (2), (3), (4);
		`)
		assert.NoError(t, err)
	})

	t.Run("UPDATE", func(t *testing.T) {
		dir := testutil.TempDir(t)

		db, err := genji.Open(filepath.Join(dir, "pebble"))
		assert.NoError(t, err)

		st, err := db.Query(`
				CREATE TABLE test;
				INSERT INTO test (a) VALUES (1), (2), (3), (4);
				UPDATE test SET a = 5;
				SELECT * FROM test;
			`)
		assert.NoError(t, err)
		defer st.Close()
		var buf bytes.Buffer
		err = testutil.IteratorToJSONArray(&buf, st)
		assert.NoError(t, err)
		require.JSONEq(t, `[{"a": 5},{"a": 5},{"a": 5},{"a": 5}]`, buf.String())
	})

	t.Run("DELETE", func(t *testing.T) {
		dir := testutil.TempDir(t)

		db, err := genji.Open(filepath.Join(dir, "pebble"))
		assert.NoError(t, err)

		err = db.Exec("CREATE TABLE test")
		assert.NoError(t, err)

		err = db.Update(func(tx *genji.Tx) error {
			for i := 1; i < 200; i++ {
				err = tx.Exec("INSERT INTO test (a) VALUES (?)", i)
				assert.NoError(t, err)
			}
			return nil
		})
		assert.NoError(t, err)

		d, err := db.QueryDocument(`
			DELETE FROM test WHERE a > 2;
			SELECT COUNT(*) FROM test;
		`)
		assert.NoError(t, err)
		var count int
		err = document.Scan(d, &count)
		assert.NoError(t, err)
		require.Equal(t, 2, count)
	})
}

// TestQueriesSameTransaction test simple queries in the same transaction.
func TestQueriesSameTransaction(t *testing.T) {
	t.Run("SELECT", func(t *testing.T) {
		dir := testutil.TempDir(t)

		db, err := genji.Open(filepath.Join(dir, "pebble"))
		assert.NoError(t, err)

		err = db.Update(func(tx *genji.Tx) error {
			d, err := tx.QueryDocument(`
				CREATE TABLE test;
				INSERT INTO test (a) VALUES (1), (2), (3), (4);
				SELECT COUNT(*) FROM test;
			`)
			assert.NoError(t, err)
			var count int
			err = document.Scan(d, &count)
			assert.NoError(t, err)
			require.Equal(t, 4, count)
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("INSERT", func(t *testing.T) {
		dir := testutil.TempDir(t)

		db, err := genji.Open(filepath.Join(dir, "pebble"))
		assert.NoError(t, err)

		err = db.Update(func(tx *genji.Tx) error {
			err = tx.Exec(`
			CREATE TABLE test;
			INSERT INTO test (a) VALUES (1), (2), (3), (4);
		`)
			assert.NoError(t, err)
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("UPDATE", func(t *testing.T) {
		dir := testutil.TempDir(t)

		db, err := genji.Open(filepath.Join(dir, "pebble"))
		assert.NoError(t, err)

		err = db.Update(func(tx *genji.Tx) error {
			st, err := tx.Query(`
				CREATE TABLE test;
				INSERT INTO test (a) VALUES (1), (2), (3), (4);
				UPDATE test SET a = 5;
				SELECT * FROM test;
			`)
			assert.NoError(t, err)
			defer st.Close()
			var buf bytes.Buffer
			err = testutil.IteratorToJSONArray(&buf, st)
			assert.NoError(t, err)
			require.JSONEq(t, `[{"a": 5},{"a": 5},{"a": 5},{"a": 5}]`, buf.String())
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("DELETE", func(t *testing.T) {
		dir := testutil.TempDir(t)

		db, err := genji.Open(filepath.Join(dir, "pebble"))
		assert.NoError(t, err)

		err = db.Update(func(tx *genji.Tx) error {
			d, err := tx.QueryDocument(`
			CREATE TABLE test;
			INSERT INTO test (a) VALUES (1), (2), (3), (4), (5), (6), (7), (8), (9), (10);
			DELETE FROM test WHERE a > 2;
			SELECT COUNT(*) FROM test;
		`)
			assert.NoError(t, err)
			var count int
			document.Scan(d, &count)
			assert.NoError(t, err)
			require.Equal(t, 2, count)
			return nil
		})
		assert.NoError(t, err)
	})
}

func TestTransient(t *testing.T) {
	ts, err := kv.NewTransientStore(&pebble.Options{FS: vfs.NewMem()})
	assert.NoError(t, err)

	dir := ts.Path

	err = ts.Put([]byte("foo"), []byte("bar"))
	assert.NoError(t, err)

	it := ts.Iterator(nil)
	defer it.Close()

	it.SeekGE([]byte("foo"))
	require.True(t, it.Valid())

	err = ts.Drop()
	assert.NoError(t, err)

	_, err = os.Stat(dir)
	require.True(t, os.IsNotExist(err))
}
