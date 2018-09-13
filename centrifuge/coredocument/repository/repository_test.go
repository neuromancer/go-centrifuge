// +build integration

package coredocumentrepository

import (
	"os"
	"testing"

	"github.com/CentrifugeInc/centrifuge-protobufs/gen/go/coredocument"
	"github.com/CentrifugeInc/go-centrifuge/centrifuge/storage"
	"github.com/CentrifugeInc/go-centrifuge/centrifuge/tools"
	"github.com/stretchr/testify/assert"
)

var dbFileName = "/tmp/centrifuge_testing_coredoc.leveldb"

func TestMain(m *testing.M) {
	levelDB := storage.NewLevelDBStorage(dbFileName)
	InitLevelDBRepository(levelDB)
	result := m.Run()
	levelDB.Close()
	os.RemoveAll(dbFileName)
	os.Exit(result)
}

var (
	id1 = tools.RandomSlice(32)
	id2 = tools.RandomSlice(32)
	id3 = tools.RandomSlice(32)
	id4 = tools.RandomSlice(32)
	id5 = tools.RandomSlice(32)
)

func TestRepository(t *testing.T) {
	repo := GetRepository()

	// failed validation for create
	doc := &coredocumentpb.CoreDocument{
		DocumentRoot:       id1,
		DocumentIdentifier: id2,
		CurrentIdentifier:  id3,
		DataRoot:           id5,
	}

	err := repo.Create(doc.DocumentIdentifier, doc)
	assert.Error(t, err, "create must fail")

	// successful creation
	doc.NextIdentifier = id4
	doc.CoredocumentSalts = &coredocumentpb.CoreDocumentSalts{
		DocumentIdentifier: id1,
		CurrentIdentifier:  id2,
		NextIdentifier:     id3,
		DataRoot:           id4,
		PreviousRoot:       id5,
	}
	err = repo.Create(doc.DocumentIdentifier, doc)
	assert.Nil(t, err, "create must pass")

	// failed get
	getDoc := new(coredocumentpb.CoreDocument)
	err = repo.GetByID(doc.NextIdentifier, getDoc)
	assert.Error(t, err, "get must fail")

	// successful get
	err = repo.GetByID(doc.DocumentIdentifier, getDoc)
	assert.Nil(t, err, "get must pass")
	assert.Equal(t, doc.DocumentIdentifier, getDoc.DocumentIdentifier, "identifiers mismatch")

	// failed update
	doc.NextIdentifier = doc.CurrentIdentifier
	err = repo.Update(doc.DocumentIdentifier, doc)
	assert.Error(t, err, "update must fail")

	// successful update
	id6 := tools.RandomSlice(32)
	doc.NextIdentifier = id6
	err = repo.Update(doc.DocumentIdentifier, doc)
	assert.Nil(t, err, "update must pass")
	err = repo.GetByID(doc.DocumentIdentifier, getDoc)
	assert.Nil(t, err, "get  must pass")
	assert.Equal(t, doc.DocumentIdentifier, getDoc.DocumentIdentifier, "identifier mismatch")
	assert.Equal(t, id6, getDoc.NextIdentifier, "identifier mismatch")
}