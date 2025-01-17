package documents

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/centrifuge/centrifuge-protobufs/gen/go/coredocument"
	"github.com/centrifuge/go-centrifuge/crypto"
	"github.com/centrifuge/go-centrifuge/errors"
	"github.com/centrifuge/go-centrifuge/identity"
	"github.com/centrifuge/go-centrifuge/utils"
	"github.com/centrifuge/precise-proofs/proofs"
	"github.com/centrifuge/precise-proofs/proofs/proto"
	"github.com/golang/protobuf/ptypes/any"
)

const (
	// CDRootField represents the coredocument root property of a tree
	CDRootField = "cd_root"

	// DataRootField represents the data root property of a tree
	DataRootField = "data_root"

	// DocumentTypeField represents the doc type property of a tree
	DocumentTypeField = "document_type"

	// SignaturesRootField represents the signatures property of a tree
	SignaturesRootField = "signatures_root"

	// SigningRootField represents the signature root property of a tree
	SigningRootField = "signing_root"

	// idSize represents the size of identifiers, roots etc..
	idSize = 32

	// nftByteCount is the length of combined bytes of registry and tokenID
	nftByteCount = 52

	// DRTreePrefix is the human readable prefix for core doc tree props
	DRTreePrefix = "dr_tree"

	// CDTreePrefix is the human readable prefix for core doc tree props
	CDTreePrefix = "cd_tree"

	// SigningTreePrefix is the human readable prefix for signing tree props
	SigningTreePrefix = "signing_tree"

	// SignaturesTreePrefix is the human readable prefix for signature props
	SignaturesTreePrefix = "signatures_tree"
)

func compactProperties(key string) []byte {
	m := map[string][]byte{
		CDRootField:         {0, 0, 0, 7},
		DataRootField:       {0, 0, 0, 5},
		DocumentTypeField:   {0, 0, 0, 100},
		SignaturesRootField: {0, 0, 0, 6},
		SigningRootField:    {0, 0, 0, 10},

		// tree prefixes use the first byte of a 4 byte slice by convention
		CDTreePrefix:         {1, 0, 0, 0},
		SigningTreePrefix:    {2, 0, 0, 0},
		SignaturesTreePrefix: {3, 0, 0, 0},
		DRTreePrefix:         {4, 0, 0, 0},
	}
	return m[key]
}

// CoreDocument is a wrapper for CoreDocument Protobuf.
type CoreDocument struct {
	Document coredocumentpb.CoreDocument
}

// newCoreDocument returns a new CoreDocument.
func newCoreDocument() (*CoreDocument, error) {
	cd := coredocumentpb.CoreDocument{
		SignatureData: new(coredocumentpb.SignatureData),
	}
	err := populateVersions(&cd, nil)
	if err != nil {
		return nil, err
	}

	return &CoreDocument{cd}, nil
}

// NewCoreDocumentFromProtobuf returns CoreDocument from the CoreDocument Protobuf.
func NewCoreDocumentFromProtobuf(cd coredocumentpb.CoreDocument) *CoreDocument {
	cd.EmbeddedDataSalts = nil
	cd.EmbeddedData = nil
	return &CoreDocument{Document: cd}
}

// NewCoreDocumentWithCollaborators generates new core Document with a document type specified by the prefix: po or invoice.
// It then adds collaborators, adds read rules and fills salts.
func NewCoreDocumentWithCollaborators(collaborators []string, documentPrefix []byte) (*CoreDocument, error) {
	cd, err := newCoreDocument()
	if err != nil {
		return nil, errors.New("failed to create coredoc: %v", err)
	}

	ids, err := identity.NewDIDsFromStrings(collaborators)
	if err != nil {
		return nil, errors.New("failed to decode collaborators: %v", err)
	}

	cd.initReadRules(ids)
	cd.initTransitionRules(ids, documentPrefix)
	if err := cd.setSalts(); err != nil {
		return nil, err
	}

	return cd, nil
}

// ID returns the Document identifier
func (cd *CoreDocument) ID() []byte {
	return cd.Document.DocumentIdentifier
}

// CurrentVersion returns the current version of the Document
func (cd *CoreDocument) CurrentVersion() []byte {
	return cd.Document.CurrentVersion
}

// CurrentVersionPreimage returns the current version preimage of the Document
func (cd *CoreDocument) CurrentVersionPreimage() []byte {
	return cd.Document.CurrentPreimage
}

// PreviousVersion returns the previous version of the Document.
func (cd *CoreDocument) PreviousVersion() []byte {
	return cd.Document.PreviousVersion
}

// NextVersion returns the next version of the Document.
func (cd *CoreDocument) NextVersion() []byte {
	return cd.Document.NextVersion
}

// PreviousDocumentRoot returns the Document root of the previous version.
func (cd *CoreDocument) PreviousDocumentRoot() []byte {
	return cd.Document.PreviousRoot
}

// AppendSignatures appends signatures to core Document.
func (cd *CoreDocument) AppendSignatures(signs ...*coredocumentpb.Signature) {
	if cd.Document.SignatureData == nil {
		cd.Document.SignatureData = new(coredocumentpb.SignatureData)
	}
	cd.Document.SignatureData.Signatures = append(cd.Document.SignatureData.Signatures, signs...)
}

// setSalts generate salts for core Document.
// This is no-op if the salts are already generated.
func (cd *CoreDocument) setSalts() error {
	if cd.Document.CoredocumentSalts != nil {
		return nil
	}

	pSalts, err := GenerateNewSalts(&cd.Document, CDTreePrefix, compactProperties(CDTreePrefix))
	if err != nil {
		return err
	}

	cd.Document.CoredocumentSalts = ConvertToProtoSalts(pSalts)
	return nil
}

// PrepareNewVersion prepares the next version of the CoreDocument
// if initSalts is true, salts will be generated for new version.
func (cd *CoreDocument) PrepareNewVersion(collaborators []string, initSalts bool, documentPrefix []byte) (*CoreDocument, error) {
	if len(cd.Document.DocumentRoot) != idSize {
		return nil, errors.New("Document root is invalid")
	}

	cs, err := identity.NewDIDsFromStrings(collaborators)
	if err != nil {
		return nil, err
	}

	// get all the old collaborators
	oldCs, err := cd.GetCollaborators()
	if err != nil {
		return nil, err
	}

	ucs := filterCollaborators(cs, oldCs...)
	cdp := coredocumentpb.CoreDocument{
		DocumentIdentifier: cd.Document.DocumentIdentifier,
		PreviousRoot:       cd.Document.DocumentRoot,
		Roles:              cd.Document.Roles,
		ReadRules:          cd.Document.ReadRules,
		TransitionRules:    cd.Document.TransitionRules,
		Nfts:               cd.Document.Nfts,
		AccessTokens:       cd.Document.AccessTokens,
		SignatureData:      new(coredocumentpb.SignatureData),
	}

	err = populateVersions(&cdp, &cd.Document)
	if err != nil {
		return nil, err
	}

	ncd := &CoreDocument{Document: cdp}
	ncd.addCollaboratorsToReadSignRules(ucs)
	ncd.addCollaboratorsToTransitionRules(ucs, documentPrefix)

	if !initSalts {
		return ncd, nil
	}

	err = ncd.setSalts()
	if err != nil {
		return nil, errors.New("failed to init salts: %v", err)
	}

	return ncd, nil
}

// newRole returns a new role with random role key
func newRole() *coredocumentpb.Role {
	return &coredocumentpb.Role{RoleKey: utils.RandomSlice(idSize)}
}

// newRoleWithCollaborators creates a new Role and adds the given collaborators to this Role.
// The Role is then returned.
// The operation returns a nil Role if no collaborators are provided.
func newRoleWithCollaborators(collaborators []identity.DID) *coredocumentpb.Role {
	if len(collaborators) == 0 {
		return nil
	}

	// create a role for given collaborators
	role := newRole()
	for _, c := range collaborators {
		c := c
		role.Collaborators = append(role.Collaborators, c[:])
	}
	return role
}

// TreeProof is a helper structure to pass to create proofs
type TreeProof struct {
	tree       *proofs.DocumentTree
	treeHashes [][]byte
}

// newTreeProof returns a TreeProof instance pointer
func newTreeProof(t *proofs.DocumentTree, th [][]byte) *TreeProof {
	return &TreeProof{tree: t, treeHashes: th}
}

// CreateProofs takes Document data tree and list to fields and generates proofs.
// we will try generating proofs from the dataTree. If failed, we will generate proofs from CoreDocument.
// errors out when the proof generation is failed on core Document tree.
func (cd *CoreDocument) CreateProofs(docType string, dataTree *proofs.DocumentTree, fields []string) (prfs []*proofspb.Proof, err error) {
	treeProofs := make(map[string]*TreeProof, 3)

	drTree, err := cd.DocumentRootTree()
	if err != nil {
		return nil, err
	}
	signatureTree, err := cd.getSignatureDataTree()
	if err != nil {
		return nil, errors.New("failed to generate signatures tree: %v", err)
	}
	cdTree, err := cd.documentTree(docType)
	if err != nil {
		return nil, errors.New("failed to generate core Document tree: %v", err)
	}
	srHash, err := cd.GetSigningRootHash()
	if err != nil {
		return nil, errors.New("failed to generate signing root proofs: %v", err)
	}

	dataRoot := dataTree.RootHash()
	cdRoot := cdTree.RootHash()

	dataPrefix, err := getDataTreePrefix(dataTree)
	if err != nil {
		return nil, err
	}

	treeProofs[DRTreePrefix] = newTreeProof(drTree, nil)
	treeProofs[dataPrefix] = newTreeProof(dataTree, append([][]byte{cdRoot}, signatureTree.RootHash()))
	treeProofs[SignaturesTreePrefix] = newTreeProof(signatureTree, [][]byte{srHash})
	treeProofs[CDTreePrefix] = newTreeProof(cdTree, append([][]byte{dataRoot}, signatureTree.RootHash()))

	return generateProofs(fields, treeProofs)
}

// TODO remove as soon as we have a public method that retrieves the parent prefix
func getDataTreePrefix(dataTree *proofs.DocumentTree) (string, error) {
	props := dataTree.PropertyOrder()
	if len(props) == 0 {
		return "", errors.New("no properties found in data tree")
	}
	fidx := strings.Split(props[0].ReadableName(), ".")
	if len(fidx) == 1 {
		return "", errors.New("no prefix found in data tree property")
	}
	return fidx[0], nil
}

// generateProofs creates proofs from fields and trees and hashes provided
func generateProofs(fields []string, treeProofs map[string]*TreeProof) (prfs []*proofspb.Proof, err error) {
	for _, f := range fields {
		fidx := strings.Split(f, ".")
		t, ok := treeProofs[fidx[0]]
		if !ok {
			return nil, errors.New("failed to find prefix tree in supported list")
		}
		tree := t.tree
		proof, err := tree.CreateProof(f)
		if err != nil {
			return nil, err
		}
		thashes := treeProofs[fidx[0]].treeHashes
		proof.SortedHashes = append(proof.SortedHashes, thashes...)
		prfs = append(prfs, &proof)
	}
	return prfs, nil
}

// GetSigningRootHash returns the hash needed to create a proof for fields from SigningRoot to DocumentRoot.
// The returned proof is appended to the proofs generated from the data tree and core Document tree for a successful verification.
func (cd *CoreDocument) GetSigningRootHash() (hash []byte, err error) {
	tree, err := cd.DocumentRootTree()
	if err != nil {
		return
	}

	rootProof, err := tree.CreateProof(fmt.Sprintf("%s.%s", DRTreePrefix, SigningRootField))
	if err != nil {
		return
	}
	return rootProof.Hash, err
}

// GetSignaturesRootHash returns the hash needed to create proofs from SignaturesRoot to DocumentRoot
func (cd *CoreDocument) GetSignaturesRootHash() (hash []byte, err error) {
	tree, err := cd.getSignatureDataTree()
	if err != nil {
		return
	}
	return tree.RootHash(), nil
}

// setSignatureDataSalts generate salts for SignatureData.
// This is no-op if the salts are already generated.
func (cd *CoreDocument) setSignatureDataSalts() ([]*coredocumentpb.DocumentSalt, error) {
	if cd.Document.SignatureDataSalts == nil {
		proofSalts, err := GenerateNewSalts(cd.Document.SignatureData, SignaturesTreePrefix, compactProperties(SignaturesTreePrefix))
		if err != nil {
			return nil, err
		}
		cd.Document.SignatureDataSalts = ConvertToProtoSalts(proofSalts)
	}
	return cd.Document.SignatureDataSalts, nil
}

// getSignatureDataTree returns the merkle tree for the Signature Data root.
func (cd *CoreDocument) getSignatureDataTree() (*proofs.DocumentTree, error) {
	signatureSalts, err := cd.setSignatureDataSalts()
	if err != nil {
		return nil, err
	}
	tree := NewDefaultTreeWithPrefix(ConvertToProofSalts(signatureSalts), SignaturesTreePrefix, compactProperties(SignaturesTreePrefix))

	err = tree.AddLeavesFromDocument(cd.Document.SignatureData)
	if err != nil {
		return nil, err
	}

	err = tree.Generate()
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// DocumentRootTree returns the merkle tree for the Document root.
func (cd *CoreDocument) DocumentRootTree() (tree *proofs.DocumentTree, err error) {
	if len(cd.Document.SigningRoot) != idSize {
		return nil, errors.New("signing root is invalid")
	}

	tree = NewDefaultTreeWithPrefix(ConvertToProofSalts(cd.Document.CoredocumentSalts), DRTreePrefix, compactProperties(DRTreePrefix))

	// The first leave added is the signing_root
	err = tree.AddLeaf(proofs.LeafNode{
		Hash:     cd.Document.SigningRoot,
		Hashed:   true,
		Property: NewLeafProperty(fmt.Sprintf("%s.%s", DRTreePrefix, SigningRootField), append(compactProperties(DRTreePrefix), compactProperties(SigningRootField)...))})
	if err != nil {
		return nil, err
	}

	// Second leaf from the signature data tree
	signatureTree, err := cd.getSignatureDataTree()
	if err != nil {
		return nil, err
	}
	err = tree.AddLeaf(proofs.LeafNode{
		Hash:     signatureTree.RootHash(),
		Hashed:   true,
		Property: NewLeafProperty(fmt.Sprintf("%s.%s", DRTreePrefix, SignaturesRootField), append(compactProperties(DRTreePrefix), compactProperties(SignaturesRootField)...))})
	if err != nil {
		return nil, err
	}

	err = tree.Generate()
	if err != nil {
		return nil, err
	}

	return tree, nil
}

// signingRootTree returns the merkle tree for the signing root.
func (cd *CoreDocument) signingRootTree(docType string) (tree *proofs.DocumentTree, err error) {
	if len(cd.Document.DataRoot) != idSize {
		return nil, errors.New("data root is invalid")
	}

	cdTree, err := cd.documentTree(docType)
	if err != nil {
		return nil, err
	}

	// create the signing tree with data root and coredoc root as siblings
	tree = NewDefaultTreeWithPrefix(ConvertToProofSalts(cd.Document.CoredocumentSalts), SigningTreePrefix, compactProperties(SigningTreePrefix))
	err = tree.AddLeaves([]proofs.LeafNode{
		{
			Property: NewLeafProperty(fmt.Sprintf("%s.%s", SigningTreePrefix, DataRootField), append(compactProperties(SigningTreePrefix), compactProperties(DataRootField)...)),
			Hash:     cd.Document.DataRoot,
			Hashed:   true,
		},
		{
			Property: NewLeafProperty(fmt.Sprintf("%s.%s", SigningTreePrefix, CDRootField), append(compactProperties(SigningTreePrefix), compactProperties(CDRootField)...)),
			Hash:     cdTree.RootHash(),
			Hashed:   true,
		},
	})

	if err != nil {
		return nil, err
	}

	err = tree.Generate()
	if err != nil {
		return nil, err
	}

	return tree, nil
}

// documentTree returns the merkle tree of the core Document.
func (cd *CoreDocument) documentTree(docType string) (tree *proofs.DocumentTree, err error) {
	tree = NewDefaultTreeWithPrefix(ConvertToProofSalts(cd.Document.CoredocumentSalts), CDTreePrefix, compactProperties(CDTreePrefix))
	err = tree.AddLeavesFromDocument(&cd.Document)
	if err != nil {
		return nil, err
	}

	dtProp := NewLeafProperty(fmt.Sprintf("%s.%s", CDTreePrefix, DocumentTypeField), append(compactProperties(CDTreePrefix), compactProperties(DocumentTypeField)...))
	// Adding document type as it is an excluded field in the tree
	documentTypeNode := proofs.LeafNode{
		Property: dtProp,
		Salt:     make([]byte, 32),
		Value:    []byte(docType),
	}

	err = documentTypeNode.HashNode(sha256.New(), true)
	if err != nil {
		return nil, err
	}

	err = tree.AddLeaf(documentTypeNode)
	if err != nil {
		return nil, err
	}

	err = tree.Generate()
	if err != nil {
		return nil, err
	}

	return tree, nil

}

// GetSignerCollaborators returns the collaborators excluding the filteredIDs
// returns collaborators with Read_Sign permissions.
func (cd *CoreDocument) GetSignerCollaborators(filterIDs ...identity.DID) ([]identity.DID, error) {
	cs, err := cd.getCollaborators(coredocumentpb.Action_ACTION_READ_SIGN)
	if err != nil {
		return nil, err
	}

	return filterCollaborators(cs, filterIDs...), nil
}

// GetCollaborators returns the collaborators excluding the filteredIDs
// returns collaborators with Read and Read_Sign permissions.
func (cd *CoreDocument) GetCollaborators(filterIDs ...identity.DID) ([]identity.DID, error) {
	cs, err := cd.getCollaborators(coredocumentpb.Action_ACTION_READ_SIGN, coredocumentpb.Action_ACTION_READ)
	if err != nil {
		return nil, err
	}

	return filterCollaborators(cs, filterIDs...), nil
}

// getCollaborators returns all the collaborators who belongs to the actions passed.
func (cd *CoreDocument) getCollaborators(actions ...coredocumentpb.Action) (ids []identity.DID, err error) {
	findRole(cd.Document, func(_, _ int, role *coredocumentpb.Role) bool {
		if len(role.Collaborators) < 1 {
			return false
		}

		for _, c := range role.Collaborators {
			// TODO(ved): we should ideally check the address length of 20
			// we will still keep the error return to the function so that once check is in, we don't have to refactor this function
			ids = append(ids, identity.NewDIDFromBytes(c))
		}

		return false
	}, actions...)

	if err != nil {
		return nil, err
	}

	return ids, nil
}

// filterCollaborators removes the filterIDs if any from cs and returns the result
func filterCollaborators(cs []identity.DID, filterIDs ...identity.DID) (filteredIDs []identity.DID) {
	filter := make(map[string]struct{})
	for _, c := range filterIDs {
		cs := strings.ToLower(c.String())
		filter[cs] = struct{}{}
	}

	for _, id := range cs {
		if _, ok := filter[strings.ToLower(id.String())]; ok {
			continue
		}

		filteredIDs = append(filteredIDs, id)
	}

	return filteredIDs
}

// CalculateDocumentRoot calculates the Document root of the core Document.
func (cd *CoreDocument) CalculateDocumentRoot() ([]byte, error) {
	tree, err := cd.DocumentRootTree()
	if err != nil {
		return nil, err
	}

	cd.Document.DocumentRoot = tree.RootHash()
	return cd.Document.DocumentRoot, nil
}

// SetDataRoot sets the document data root to core document.
func (cd *CoreDocument) SetDataRoot(dr []byte) {
	cd.Document.DataRoot = dr
}

// CalculateSigningRoot calculates the signing root of the core Document.
func (cd *CoreDocument) CalculateSigningRoot(docType string) ([]byte, error) {
	tree, err := cd.signingRootTree(docType)
	if err != nil {
		return nil, err
	}

	cd.Document.SigningRoot = tree.RootHash()
	return cd.Document.SigningRoot, nil
}

// PackCoreDocument prepares the document into a core document.
func (cd *CoreDocument) PackCoreDocument(data *any.Any, salts []*coredocumentpb.DocumentSalt) coredocumentpb.CoreDocument {
	// lets copy the value so that mutations on the returned doc wont be reflected on Document we are holding
	cdp := cd.Document
	cdp.EmbeddedData = data
	cdp.EmbeddedDataSalts = salts
	return cdp
}

// Signatures returns the copy of the signatures on the Document.
func (cd *CoreDocument) Signatures() (signatures []coredocumentpb.Signature) {
	for _, s := range cd.Document.SignatureData.Signatures {
		signatures = append(signatures, *s)
	}

	return signatures
}

// AddUpdateLog adds a log to the model to persist an update related meta data such as author
func (cd *CoreDocument) AddUpdateLog(account identity.DID) (err error) {
	cd.Document.Author = account[:]
	cd.Document.Timestamp, err = utils.ToTimestamp(time.Now().UTC())
	if err != nil {
		return err
	}
	return nil
}

// Author is the author of the document version represented by the model
func (cd *CoreDocument) Author() identity.DID {
	return identity.NewDIDFromBytes(cd.Document.Author)
}

// Timestamp is the time of update in UTC of the document version represented by the model
func (cd *CoreDocument) Timestamp() (time.Time, error) {
	return utils.FromTimestamp(cd.Document.Timestamp)
}

func populateVersions(cd *coredocumentpb.CoreDocument, prevCD *coredocumentpb.CoreDocument) (err error) {
	if prevCD != nil {
		cd.PreviousVersion = prevCD.CurrentVersion
		cd.CurrentVersion = prevCD.NextVersion
		cd.CurrentPreimage = prevCD.NextPreimage
	} else {
		cd.CurrentPreimage, cd.CurrentVersion, err = crypto.GenerateHashPair(idSize)
		cd.DocumentIdentifier = cd.CurrentVersion
		if err != nil {
			return err
		}
	}
	cd.NextPreimage, cd.NextVersion, err = crypto.GenerateHashPair(idSize)
	if err != nil {
		return err
	}
	return nil
}
