// +build unit

package documents

import (
	"fmt"
	"testing"

	"github.com/centrifuge/centrifuge-protobufs/documenttypes"
	"github.com/centrifuge/centrifuge-protobufs/gen/go/coredocument"
	"github.com/centrifuge/centrifuge-protobufs/gen/go/p2p"
	"github.com/centrifuge/go-centrifuge/contextutil"
	"github.com/centrifuge/go-centrifuge/errors"
	"github.com/centrifuge/go-centrifuge/identity"
	"github.com/centrifuge/go-centrifuge/protobufs/gen/go/document"
	"github.com/centrifuge/go-centrifuge/protobufs/gen/go/invoice"
	"github.com/centrifuge/go-centrifuge/testingutils/commons"
	"github.com/centrifuge/go-centrifuge/testingutils/config"
	"github.com/centrifuge/go-centrifuge/testingutils/identity"
	"github.com/centrifuge/go-centrifuge/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestReadACLs_initReadRules(t *testing.T) {
	cd, err := newCoreDocument()
	assert.NoError(t, err)
	cd.initReadRules(nil)
	assert.Nil(t, cd.Document.Roles)
	assert.Nil(t, cd.Document.ReadRules)

	cs := []identity.DID{testingidentity.GenerateRandomDID()}
	cd.initReadRules(cs)
	assert.Len(t, cd.Document.ReadRules, 1)
	assert.Len(t, cd.Document.Roles, 1)

	cd.initReadRules(cs)
	assert.Len(t, cd.Document.ReadRules, 1)
	assert.Len(t, cd.Document.Roles, 1)
}

func TestReadAccessValidator_AccountCanRead(t *testing.T) {
	cd, err := newCoreDocument()
	assert.NoError(t, err)
	account := testingidentity.GenerateRandomDID()
	cd.Document.DocumentRoot = utils.RandomSlice(32)
	ncd, err := cd.PrepareNewVersion([]string{account.String()}, false, nil)
	assert.NoError(t, err)
	assert.NotNil(t, ncd.Document.ReadRules)
	assert.NotNil(t, ncd.Document.Roles)

	// account who cant access
	rcid := testingidentity.GenerateRandomDID()
	assert.False(t, ncd.AccountCanRead(rcid))

	// account can access
	assert.True(t, ncd.AccountCanRead(account))
}

type mockRegistry struct {
	mock.Mock
}

func (m mockRegistry) OwnerOf(registry common.Address, tokenID []byte) (common.Address, error) {
	args := m.Called(registry, tokenID)
	addr, _ := args.Get(0).(common.Address)
	return addr, args.Error(1)
}

func TestCoreDocument_addNFTToReadRules(t *testing.T) {
	cd, err := newCoreDocument()
	assert.NoError(t, err)

	// wrong registry or token format
	registry := common.HexToAddress("0xf72855759a39fb75fc7341139f5d7a3974d4da08")
	tokenID := utils.RandomSlice(34)
	err = cd.addNFTToReadRules(registry, tokenID)
	assert.Error(t, err)
	assert.Nil(t, cd.Document.CoredocumentSalts)
	assert.Nil(t, cd.Document.ReadRules)
	assert.Nil(t, cd.Document.Roles)

	tokenID = utils.RandomSlice(32)
	err = cd.addNFTToReadRules(registry, tokenID)
	assert.NoError(t, err)
	assert.NotNil(t, cd.Document.CoredocumentSalts)
	assert.Len(t, cd.Document.ReadRules, 1)
	assert.Equal(t, cd.Document.ReadRules[0].Action, coredocumentpb.Action_ACTION_READ)
	assert.Len(t, cd.Document.Roles, 1)
	enft, err := ConstructNFT(registry, tokenID)
	assert.NoError(t, err)
	assert.Equal(t, enft, cd.Document.Roles[0].Nfts[0])
}

func TestCoreDocument_NFTOwnerCanRead(t *testing.T) {
	account := testingidentity.GenerateRandomDID()
	cd, err := NewCoreDocumentWithCollaborators([]string{account.String()}, nil)
	assert.NoError(t, err)
	registry := common.HexToAddress("0xf72855759a39fb75fc7341139f5d7a3974d4da08")

	// account can read
	assert.NoError(t, cd.NFTOwnerCanRead(nil, registry, nil, account))

	// account not in read rules and nft missing
	account = testingidentity.GenerateRandomDID()
	tokenID := utils.RandomSlice(32)
	assert.Error(t, cd.NFTOwnerCanRead(nil, registry, tokenID, account))

	tr := mockRegistry{}
	tr.On("OwnerOf", registry, tokenID).Return(nil, errors.New("failed to get owner of")).Once()
	assert.NoError(t, cd.addNFTToReadRules(registry, tokenID))
	assert.Error(t, cd.NFTOwnerCanRead(tr, registry, tokenID, account))
	tr.AssertExpectations(t)

	// not the same owner
	owner := common.BytesToAddress(utils.RandomSlice(20))
	tr.On("OwnerOf", registry, tokenID).Return(owner, nil).Once()
	assert.Error(t, cd.NFTOwnerCanRead(tr, registry, tokenID, account))
	tr.AssertExpectations(t)

	// same owner
	owner = account.ToAddress()
	tr.On("OwnerOf", registry, tokenID).Return(owner, nil).Once()
	assert.NoError(t, cd.NFTOwnerCanRead(tr, registry, tokenID, account))
	tr.AssertExpectations(t)
}

func TestCoreDocumentModel_AddNFT(t *testing.T) {
	cd, err := newCoreDocument()
	assert.NoError(t, err)
	cd.Document.DocumentRoot = utils.RandomSlice(32)
	registry := common.HexToAddress("0xf72855759a39fb75fc7341139f5d7a3974d4da08")
	registry2 := common.HexToAddress("0xf72855759a39fb75fc7341139f5d7a3974d4da02")
	tokenID := utils.RandomSlice(32)
	assert.Nil(t, cd.Document.Nfts)
	assert.Nil(t, cd.Document.ReadRules)
	assert.Nil(t, cd.Document.Roles)

	cd, err = cd.AddNFT(true, registry, tokenID)
	assert.Nil(t, err)
	assert.Len(t, cd.Document.Nfts, 1)
	assert.Len(t, cd.Document.Nfts[0].RegistryId, 32)
	assert.Equal(t, tokenID, getStoredNFT(cd.Document.Nfts, registry.Bytes()).TokenId)
	assert.Nil(t, getStoredNFT(cd.Document.Nfts, registry2.Bytes()))
	assert.Len(t, cd.Document.ReadRules, 1)
	assert.Len(t, cd.Document.Roles, 1)
	assert.Len(t, cd.Document.Roles[0].Nfts, 1)

	tokenID = utils.RandomSlice(32)
	cd.Document.DocumentRoot = utils.RandomSlice(32)
	cd, err = cd.AddNFT(true, registry, tokenID)
	assert.Nil(t, err)
	assert.Len(t, cd.Document.Nfts, 1)
	assert.Len(t, cd.Document.Nfts[0].RegistryId, 32)
	assert.Equal(t, tokenID, getStoredNFT(cd.Document.Nfts, registry.Bytes()).TokenId)
	assert.Nil(t, getStoredNFT(cd.Document.Nfts, registry2.Bytes()))
	assert.Len(t, cd.Document.ReadRules, 2)
	assert.Len(t, cd.Document.Roles, 2)
	assert.Len(t, cd.Document.Roles[1].Nfts, 1)
}

func TestCoreDocument_IsNFTMinted(t *testing.T) {
	cd, err := newCoreDocument()
	assert.NoError(t, err)
	registry := common.HexToAddress("0xf72855759a39fb75fc7341139f5d7a3974d4da08")
	assert.False(t, cd.IsNFTMinted(nil, registry))

	cd.Document.DocumentRoot = utils.RandomSlice(32)
	tokenID := utils.RandomSlice(32)
	owner := common.HexToAddress("0xf72855759a39fb75fc7341139f5d7a3974d4da02")
	cd, err = cd.AddNFT(true, registry, tokenID)
	assert.Nil(t, err)

	tr := new(mockRegistry)
	tr.On("OwnerOf", registry, tokenID).Return(owner, nil).Once()
	assert.True(t, cd.IsNFTMinted(tr, registry))
	tr.AssertExpectations(t)
}

func TestCoreDocument_getReadAccessProofKeys(t *testing.T) {
	cd, err := newCoreDocument()
	assert.NoError(t, err)
	registry := common.HexToAddress("0xf72855759a39fb75fc7341139f5d7a3974d4da08")
	tokenID := utils.RandomSlice(32)

	pfs, err := getReadAccessProofKeys(cd.Document, registry, tokenID)
	assert.Error(t, err)
	assert.Nil(t, pfs)

	cd.Document.DocumentRoot = utils.RandomSlice(32)
	cd, err = cd.AddNFT(true, registry, tokenID)
	assert.NoError(t, err)
	assert.NotNil(t, cd)

	pfs, err = getReadAccessProofKeys(cd.Document, registry, tokenID)
	assert.NoError(t, err)
	assert.Len(t, pfs, 3)
	assert.Equal(t, CDTreePrefix+".read_rules[0].roles[0]", pfs[0])
	assert.Equal(t, CDTreePrefix+".read_rules[0].action", pfs[1])
	assert.Equal(t, fmt.Sprintf(CDTreePrefix+".roles[%s].nfts[0]", hexutil.Encode(cd.Document.Roles[0].RoleKey)), pfs[2])
}

func TestCoreDocument_getNFTUniqueProofKey(t *testing.T) {
	cd, err := newCoreDocument()
	assert.NoError(t, err)
	registry := common.HexToAddress("0xf72855759a39fb75fc7341139f5d7a3974d4da08")
	pf, err := getNFTUniqueProofKey(cd.Document.Nfts, registry)
	assert.Error(t, err)
	assert.Empty(t, pf)

	cd.Document.DocumentRoot = utils.RandomSlice(32)
	tokenID := utils.RandomSlice(32)
	cd, err = cd.AddNFT(false, registry, tokenID)
	assert.NoError(t, err)
	assert.NotNil(t, cd)

	pf, err = getNFTUniqueProofKey(cd.Document.Nfts, registry)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf(CDTreePrefix+".nfts[%s]", hexutil.Encode(append(registry.Bytes(), make([]byte, 12, 12)...))), pf)
}

func TestCoreDocument_getRoleProofKey(t *testing.T) {
	cd, err := newCoreDocument()
	assert.NoError(t, err)
	roleKey := make([]byte, 32, 32)
	account := testingidentity.GenerateRandomDID()
	pf, err := getRoleProofKey(cd.Document.Roles, roleKey, account)
	assert.Error(t, err)
	assert.Empty(t, pf)

	cd.initReadRules([]identity.DID{account})
	roleKey = cd.Document.Roles[0].RoleKey
	pf, err = getRoleProofKey(cd.Document.Roles, roleKey, testingidentity.GenerateRandomDID())
	assert.Error(t, err)
	assert.True(t, errors.IsOfType(ErrNFTRoleMissing, err))
	assert.Empty(t, pf)

	pf, err = getRoleProofKey(cd.Document.Roles, roleKey, account)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf(CDTreePrefix+".roles[%s].collaborators[0]", hexutil.Encode(roleKey)), pf)
}

func TestCoreDocumentModel_GetNFTProofs(t *testing.T) {
	cd, err := newCoreDocument()
	assert.NoError(t, err)
	invData := &invoicepb.InvoiceData{}
	dataSalts, err := GenerateNewSalts(invData, "invoice", []byte{1, 0, 0, 0})
	assert.NoError(t, err)

	cd.Document.DataRoot = utils.RandomSlice(32)
	cd.Document.EmbeddedData = &any.Any{Value: utils.RandomSlice(32), TypeUrl: documenttypes.InvoiceDataTypeUrl}
	account := testingidentity.GenerateRandomDID()
	cd.initReadRules([]identity.DID{account})
	registry := common.HexToAddress("0xf72855759a39fb75fc7341139f5d7a3974d4da08")
	tokenID := utils.RandomSlice(32)
	cd.Document.EmbeddedDataSalts = ConvertToProtoSalts(dataSalts)
	assert.NoError(t, err)
	assert.NoError(t, cd.setSalts())
	_, err = cd.CalculateSigningRoot(documenttypes.InvoiceDataTypeUrl)
	assert.NoError(t, err)
	_, err = cd.CalculateDocumentRoot()
	assert.NoError(t, err)
	cd, err = cd.AddNFT(true, registry, tokenID)
	assert.NoError(t, err)
	cd.Document.DataRoot = utils.RandomSlice(32)
	assert.NoError(t, cd.setSalts())
	_, err = cd.CalculateSigningRoot(documenttypes.InvoiceDataTypeUrl)
	assert.NoError(t, err)
	_, err = cd.CalculateDocumentRoot()
	assert.NoError(t, err)

	tests := []struct {
		registry       common.Address
		tokenID        []byte
		nftReadAccess  bool
		nftUniqueProof bool
		error          bool
	}{

		// failed nft unique proof
		{
			nftUniqueProof: true,
			registry:       common.BytesToAddress(utils.RandomSlice(20)),
			error:          true,
		},

		// good nft unique proof
		{
			nftUniqueProof: true,
			registry:       registry,
		},

		// failed read access proof
		{
			nftReadAccess: true,
			registry:      registry,
			tokenID:       utils.RandomSlice(32),
			error:         true,
		},

		// good read access proof
		{
			nftReadAccess: true,
			registry:      registry,
			tokenID:       tokenID,
		},

		// all proofs
		{
			nftUniqueProof: true,
			registry:       registry,
			nftReadAccess:  true,
			tokenID:        tokenID,
		},
	}

	tree, err := cd.DocumentRootTree()
	assert.NoError(t, err)

	for _, c := range tests {
		pfs, err := cd.CreateNFTProofs(documenttypes.InvoiceDataTypeUrl, account, c.registry, c.tokenID, c.nftUniqueProof, c.nftReadAccess)
		if c.error {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		assert.True(t, len(pfs) > 0)

		for _, pf := range pfs {
			valid, err := tree.ValidateProof(pf)
			assert.NoError(t, err)
			assert.True(t, valid)
		}
	}
}

func TestCoreDocumentModel_ATOwnerCanRead(t *testing.T) {
	ctx := testingconfig.CreateAccountContext(t, cfg)
	account, _ := contextutil.Account(ctx)
	srv := new(testingcommons.MockIdentityService)
	id, err := account.GetIdentityID()
	granteeID, err := identity.NewDIDFromString("0xBAEb33a61f05e6F269f1c4b4CFF91A901B54DaF7")
	assert.NoError(t, err)
	granterID := identity.NewDIDFromBytes(id)
	assert.NoError(t, err)
	cd, err := NewCoreDocumentWithCollaborators([]string{granterID.String()}, nil)
	assert.NoError(t, err)
	cd.Document.DocumentRoot = utils.RandomSlice(32)
	payload := documentpb.AccessTokenParams{
		Grantee:            hexutil.Encode(granteeID[:]),
		DocumentIdentifier: hexutil.Encode(cd.Document.DocumentIdentifier),
	}
	ncd, err := cd.AddAccessToken(ctx, payload)
	assert.NoError(t, err)
	ncd.Document.DocumentRoot = utils.RandomSlice(32)
	at := ncd.Document.AccessTokens[0]
	assert.NotNil(t, at)
	// wrong token identifier
	tr := &p2ppb.AccessTokenRequest{
		DelegatingDocumentIdentifier: ncd.Document.DocumentIdentifier,
		AccessTokenId:                []byte("randomtokenID"),
	}
	dr := &p2ppb.GetDocumentRequest{
		DocumentIdentifier: ncd.Document.DocumentIdentifier,
		AccessType:         p2ppb.AccessType_ACCESS_TYPE_ACCESS_TOKEN_VERIFICATION,
		AccessTokenRequest: tr,
	}
	err = ncd.ATGranteeCanRead(ctx, srv, dr.AccessTokenRequest.AccessTokenId, dr.DocumentIdentifier, granteeID)
	assert.Error(t, err, "access token not found")
	// invalid signing key
	tr = &p2ppb.AccessTokenRequest{
		DelegatingDocumentIdentifier: ncd.Document.DocumentIdentifier,
		AccessTokenId:                at.Identifier,
	}
	dr.AccessTokenRequest = tr
	srv.On("ValidateKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("key not linked to identity")).Once()
	err = ncd.ATGranteeCanRead(ctx, srv, dr.AccessTokenRequest.AccessTokenId, dr.DocumentIdentifier, granteeID)
	assert.Error(t, err)
	// valid key
	srv.On("ValidateKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	err = ncd.ATGranteeCanRead(ctx, srv, dr.AccessTokenRequest.AccessTokenId, dr.DocumentIdentifier, granteeID)
	assert.NoError(t, err)
}

func TestCoreDocumentModel_AddAccessToken(t *testing.T) {
	m, err := newCoreDocument()
	assert.NoError(t, err)
	m.Document.DocumentRoot = utils.RandomSlice(32)
	ctx := testingconfig.CreateAccountContext(t, cfg)
	account, err := contextutil.Account(ctx)
	assert.NoError(t, err)

	cd := m.Document
	assert.Len(t, cd.AccessTokens, 0)

	// invalid centID format
	payload := documentpb.AccessTokenParams{
		// invalid grantee format
		Grantee:            "randomCentID",
		DocumentIdentifier: "randomDocID",
	}
	_, err = m.AddAccessToken(ctx, payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to construct access token: malformed address provided")
	// invalid centID length
	invalidCentID := utils.RandomSlice(25)
	payload = documentpb.AccessTokenParams{
		Grantee:            hexutil.Encode(invalidCentID),
		DocumentIdentifier: hexutil.Encode(m.Document.DocumentIdentifier),
	}
	_, err = m.AddAccessToken(ctx, payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to construct access token: malformed address provided")
	// invalid docID length
	id, err := account.GetIdentityID()
	assert.NoError(t, err)
	invalidDocID := utils.RandomSlice(33)
	payload = documentpb.AccessTokenParams{
		Grantee:            hexutil.Encode(id),
		DocumentIdentifier: hexutil.Encode(invalidDocID),
	}

	_, err = m.AddAccessToken(ctx, payload)
	assert.Contains(t, err.Error(), "failed to construct access token: invalid identifier length")
	// valid
	payload = documentpb.AccessTokenParams{
		Grantee:            hexutil.Encode(id),
		DocumentIdentifier: hexutil.Encode(m.Document.DocumentIdentifier),
	}

	ncd, err := m.AddAccessToken(ctx, payload)
	assert.NoError(t, err)
	assert.Len(t, ncd.Document.AccessTokens, 1)
}
