package documents

import (
	"context"
	"time"

	"github.com/centrifuge/centrifuge-protobufs/gen/go/coredocument"
	"github.com/centrifuge/go-centrifuge/identity"
	"github.com/centrifuge/go-centrifuge/storage"
	"github.com/centrifuge/precise-proofs/proofs/proto"
	"github.com/ethereum/go-ethereum/common"
)

// Model is an interface to abstract away model specificness like invoice or purchaseOrder
// The interface can cast into the type specified by the model if required
// It should only handle protocol-level Document actions
type Model interface {
	storage.Model

	// ID returns the Document identifier
	ID() []byte

	// CurrentVersion returns the current version identifier of the Document
	CurrentVersion() []byte

	// CurrentVersionPreimage returns the current version pre-image of the Document. This is intended to hide the next version of an updated version of the document.
	CurrentVersionPreimage() []byte

	// PreviousVersion returns the previous version identifier of the Document
	PreviousVersion() []byte

	// NextVersion returns the next version identifier of the Document.
	NextVersion() []byte

	// PackCoreDocument packs the implementing Document into a core Document
	// Should only be called when the Document is about to be put on wire.
	PackCoreDocument() (coredocumentpb.CoreDocument, error)

	// UnpackCoreDocument takes a core Document protobuf and loads the data into the model.
	UnpackCoreDocument(cd coredocumentpb.CoreDocument) error

	// DocumentType returns the type of the Document
	DocumentType() string

	// CalculateDataRoot calculates the data root of the model.
	CalculateDataRoot() ([]byte, error)

	// CalculateSigningRoot calculates the signing root of the model.
	CalculateSigningRoot() ([]byte, error)

	// CalculateDocumentRoot returns the Document root of the model.
	CalculateDocumentRoot() ([]byte, error)

	// GetSigningRootHash get the hash for signing root of the model.
	GetSigningRootHash() (hash []byte, err error)

	// GetSignaturesRootHash get hash for the signatures root of the model
	GetSignaturesRootHash() (hash []byte, err error)

	// PreviousDocumentRoot returns the Document root of the previous version.
	PreviousDocumentRoot() []byte

	// AppendSignatures appends the signatures to the model.
	AppendSignatures(signatures ...*coredocumentpb.Signature)

	// Signatures returns a copy of the signatures on the Document
	Signatures() []coredocumentpb.Signature

	// CreateProofs creates precise-proofs for given fields
	CreateProofs(fields []string) (proofs []*proofspb.Proof, err error)

	// CreateNFTProofs creates NFT proofs for minting.
	CreateNFTProofs(
		account identity.DID,
		registry common.Address,
		tokenID []byte,
		nftUniqueProof, readAccessProof bool) (proofs []*proofspb.Proof, err error)

	// IsNFTMinted checks if there is any NFT minted for the registry given
	IsNFTMinted(tr TokenRegistry, registry common.Address) bool

	// AddNFT adds an NFT to the Document.
	// Note: The Document should be anchored after successfully adding the NFT.
	AddNFT(grantReadAccess bool, registry common.Address, tokenID []byte) error

	// GetCollaborators returns the collaborators of this Document.
	// filter ids should not be returned
	// Note: returns all the collaborators with Read and Read_Sign permission
	GetCollaborators(filterIDs ...identity.DID) ([]identity.DID, error)

	// GetSignerCollaborators works like GetCollaborators except it returns only those with Read_Sign permission.
	GetSignerCollaborators(filterIDs ...identity.DID) ([]identity.DID, error)

	// AccountCanRead returns true if the account can read the document
	AccountCanRead(account identity.DID) bool

	// NFTOwnerCanRead returns error if the NFT cannot read the document.
	NFTOwnerCanRead(tokenRegistry TokenRegistry, registry common.Address, tokenID []byte, account identity.DID) error

	// ATGranteeCanRead returns error if the access token grantee cannot read the document.
	ATGranteeCanRead(ctx context.Context, idSrv identity.ServiceDID, tokenID, docID []byte, grantee identity.DID) (err error)

	// AddUpdateLog adds a log to the model to persist an update related meta data such as author
	AddUpdateLog(account identity.DID) error

	// Author is the author of the document version represented by the model
	Author() identity.DID

	// Timestamp is the time of update in UTC of the document version represented by the model
	Timestamp() (time.Time, error)

	// CollaboratorCanUpdate returns an error if indicated identity does not have the capacity to update the document.
	CollaboratorCanUpdate(updated Model, collaborator identity.DID) error
}

// TokenRegistry defines NFT related functions.
type TokenRegistry interface {
	// OwnerOf to retrieve owner of the tokenID
	OwnerOf(registry common.Address, tokenID []byte) (common.Address, error)
}
