// This file is part of the go-meta library.
//
// Copyright (C) 2017 JAAK MUSIC LTD
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//
// If you have any questions please contact yo@jaak.io

package musicbrainz

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/meta-network/go-meta"
)

// TestIndexArtists tests indexing a stream of MusicBrainz artists.
func TestIndexArtists(t *testing.T) {
	x, err := newTestIndex()
	if err != nil {
		t.Fatal(err)
	}
	defer x.cleanup()

	// check all the artists were indexed
	for _, artist := range x.artists {
		// check the name, type and mbid indexes
		rows, err := x.db.Query(
			`SELECT object_id FROM artist WHERE name = ? AND type = ? AND mbid = ?`,
			artist.Name, artist.Type, artist.MBID,
		)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var objectID string
		for rows.Next() {
			// if we've already set objectID then we have a duplicate
			if objectID != "" {
				t.Fatalf("duplicate entries for artist %q", artist.Name)
			}
			if err := rows.Scan(&objectID); err != nil {
				t.Fatal(err)
			}

			// check we can get the object from the store
			cid, err := cid.Parse(objectID)
			if err != nil {
				t.Fatal(err)
			}
			obj, err := x.store.Get(cid)
			if err != nil {
				t.Fatal(err)
			}

			// check the object has the correct fields
			for key, expected := range map[string]string{
				"name": artist.Name,
				"type": artist.Type,
				"mbid": artist.MBID,
			} {
				actual, err := obj.GetString(key)
				if err != nil {
					t.Fatal(err)
				}
				if actual != expected {
					t.Fatalf("expected object %s to be %q, got %q", key, expected, actual)
				}
			}
		}

		// check we got an object and no db errors
		if objectID == "" {
			t.Fatalf("artist %q not found", artist.Name)
		} else if err := rows.Err(); err != nil {
			t.Fatal(err)
		}

		// check the IPI index
		if len(artist.IPI) > 0 {
			var ipis []string
			rows, err = x.db.Query(
				`SELECT ipi FROM artist_ipi WHERE object_id = ?`,
				objectID,
			)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			for rows.Next() {
				var ipi string
				if err := rows.Scan(&ipi); err != nil {
					t.Fatal(err)
				}
				ipis = append(ipis, ipi)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ipis, artist.IPI) {
				t.Fatalf("expected %q to have %d IPIs, got %d:\nexpected: %v\nactual   %v", artist.Name, len(artist.IPI), len(ipis), artist.IPI, ipis)
			}
		}

		// check the ISNI index
		if len(artist.ISNI) > 0 {
			var isnis []string
			rows, err = x.db.Query(
				`SELECT isni FROM artist_isni WHERE object_id = ?`,
				objectID,
			)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			for rows.Next() {
				var isni string
				if err := rows.Scan(&isni); err != nil {
					t.Fatal(err)
				}
				isnis = append(isnis, isni)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(isnis, artist.ISNI) {
				t.Fatalf("expected %q to have %d ISNIs, got %d:\nexpected: %v\nactual   %v", artist.Name, len(artist.IPI), len(isnis), artist.IPI, isnis)
			}
		}
	}
}

type testIndex struct {
	db      *sql.DB
	store   *meta.Store
	artists []*Artist
	tmpDir  string
}

func (t *testIndex) cleanup() {
	if t.db != nil {
		t.db.Close()
	}
	if t.tmpDir != "" {
		os.RemoveAll(t.tmpDir)
	}
}

func newTestIndex() (x *testIndex, err error) {
	x = &testIndex{}
	defer func() {
		if err != nil {
			x.cleanup()
		}
	}()

	// load the test artists
	f, err := os.Open("testdata/artists.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for {
		var artist *Artist
		err := dec.Decode(&artist)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		x.artists = append(x.artists, artist)
	}

	// store the artists in a test store
	x.store = meta.NewStore(datastore.NewMapDatastore())
	cids := make([]*cid.Cid, len(x.artists))
	for i, artist := range x.artists {
		obj, err := meta.Encode(artist)
		if err != nil {
			return nil, err
		}
		if err := x.store.Put(obj); err != nil {
			return nil, err
		}
		cids[i] = obj.Cid()
	}

	// create a stream
	stream := make(chan *cid.Cid, len(x.artists))
	go func() {
		defer close(stream)
		for _, cid := range cids {
			stream <- cid
		}
	}()

	// create a test SQLite3 db
	x.tmpDir, err = ioutil.TempDir("", "musicbrainz-index-test")
	if err != nil {
		return nil, err
	}
	x.db, err = sql.Open("sqlite3", filepath.Join(x.tmpDir, "index.db"))
	if err != nil {
		return nil, err
	}

	// index the artists
	indexer, err := NewIndexer(x.db, x.store)
	if err != nil {
		return nil, err
	}
	if err := indexer.IndexArtists(context.Background(), stream); err != nil {
		return nil, err
	}
	return x, nil

}