package documents

import (
	"crypto/sha256"

	"github.com/centrifuge/centrifuge-protobufs/gen/go/coredocument"
	"github.com/centrifuge/precise-proofs/proofs"
	"github.com/gogo/protobuf/proto"
)

// NewDefaultTree returns a DocumentTree with default opts
func NewDefaultTree(salts *proofs.Salts) *proofs.DocumentTree {
	return NewDefaultTreeWithPrefix(salts, "", nil)
}

// NewDefaultTreeWithPrefix returns a DocumentTree with default opts passing a prefix to the tree leaves
func NewDefaultTreeWithPrefix(salts *proofs.Salts, prefix string, compactPrefix []byte) *proofs.DocumentTree {
	var prop proofs.Property
	if prefix != "" {
		prop = NewLeafProperty(prefix, compactPrefix)
	}

	t := proofs.NewDocumentTree(proofs.TreeOptions{CompactProperties: true, EnableHashSorting: true, Hash: sha256.New(), ParentPrefix: prop})
	return &t
}

// NewLeafProperty returns a proof property with the literal and the compact
func NewLeafProperty(literal string, compact []byte) proofs.Property {
	return proofs.NewProperty(literal, compact...)
}

// GenerateNewSalts generates salts for new Document
func GenerateNewSalts(document proto.Message, prefix string, compactPrefix []byte) (*proofs.Salts, error) {
	docSalts := new(proofs.Salts)
	t := NewDefaultTreeWithPrefix(docSalts, prefix, compactPrefix)
	err := t.AddLeavesFromDocument(document)
	if err != nil {
		return nil, err
	}
	return docSalts, nil
}
