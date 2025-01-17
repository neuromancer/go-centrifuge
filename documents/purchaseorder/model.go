package purchaseorder

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/centrifuge/centrifuge-protobufs/documenttypes"
	"github.com/centrifuge/centrifuge-protobufs/gen/go/coredocument"
	"github.com/centrifuge/centrifuge-protobufs/gen/go/purchaseorder"
	"github.com/centrifuge/go-centrifuge/centerrors"
	"github.com/centrifuge/go-centrifuge/documents"
	"github.com/centrifuge/go-centrifuge/errors"
	"github.com/centrifuge/go-centrifuge/identity"
	clientpurchaseorderpb "github.com/centrifuge/go-centrifuge/protobufs/gen/go/purchaseorder"
	"github.com/centrifuge/precise-proofs/proofs"
	"github.com/centrifuge/precise-proofs/proofs/proto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/timestamp"
)

const prefix string = "po"

// tree prefixes for specific to documents use the second byte of a 4 byte slice by convention
func compactPrefix() []byte { return []byte{0, 2, 0, 0} }

// PurchaseOrder implements the documents.Model keeps track of purchase order related fields and state
type PurchaseOrder struct {
	*documents.CoreDocument
	Status             string // status of the Purchase Order
	PoNumber           string // purchase order number or reference number
	OrderName          string // name of the ordering company
	OrderStreet        string // street and address details of the ordering company
	OrderCity          string
	OrderZipcode       string
	OrderCountry       string // country ISO code of the ordering company of this purchase order
	RecipientName      string // name of the recipient company
	RecipientStreet    string
	RecipientCity      string
	RecipientZipcode   string
	RecipientCountry   string // country ISO code of the recipient of this purchase order
	Currency           string // ISO currency code
	OrderAmount        int64  // ordering gross amount including tax
	NetAmount          int64  // invoice amount excluding tax
	TaxAmount          int64
	TaxRate            int64
	Recipient          *identity.DID
	Order              []byte
	OrderContact       string
	Comment            string
	DeliveryDate       *timestamp.Timestamp // requested delivery date
	DateCreated        *timestamp.Timestamp // purchase order date
	ExtraData          []byte
	PurchaseOrderSalts *proofs.Salts
}

// getClientData returns the client data from the purchaseOrder model
func (p *PurchaseOrder) getClientData() *clientpurchaseorderpb.PurchaseOrderData {
	var recipient string
	if p.Recipient != nil {
		recipient = hexutil.Encode(p.Recipient[:])
	}

	var order string
	if p.Order != nil {
		order = hexutil.Encode(p.Order)
	}

	var extraData string
	if p.ExtraData != nil {
		extraData = hexutil.Encode(p.ExtraData)
	}

	return &clientpurchaseorderpb.PurchaseOrderData{
		PoStatus:         p.Status,
		PoNumber:         p.PoNumber,
		OrderName:        p.OrderName,
		OrderStreet:      p.OrderStreet,
		OrderCity:        p.OrderCity,
		OrderZipcode:     p.OrderZipcode,
		OrderCountry:     p.OrderCountry,
		RecipientName:    p.RecipientName,
		RecipientStreet:  p.RecipientStreet,
		RecipientCity:    p.RecipientCity,
		RecipientZipcode: p.RecipientZipcode,
		RecipientCountry: p.RecipientCountry,
		Currency:         p.Currency,
		OrderAmount:      p.OrderAmount,
		NetAmount:        p.NetAmount,
		TaxAmount:        p.TaxAmount,
		TaxRate:          p.TaxRate,
		Recipient:        recipient,
		Order:            order,
		OrderContact:     p.OrderContact,
		Comment:          p.Comment,
		DeliveryDate:     p.DeliveryDate,
		DateCreated:      p.DateCreated,
		ExtraData:        extraData,
	}

}

// createP2PProtobuf returns centrifuge protobuf specific purchaseOrderData
func (p *PurchaseOrder) createP2PProtobuf() *purchaseorderpb.PurchaseOrderData {
	var recipient []byte
	if p.Recipient != nil {
		recipient = p.Recipient[:]
	}

	return &purchaseorderpb.PurchaseOrderData{
		PoStatus:         p.Status,
		PoNumber:         p.PoNumber,
		OrderName:        p.OrderName,
		OrderStreet:      p.OrderStreet,
		OrderCity:        p.OrderCity,
		OrderZipcode:     p.OrderZipcode,
		OrderCountry:     p.OrderCountry,
		RecipientName:    p.RecipientName,
		RecipientStreet:  p.RecipientStreet,
		RecipientCity:    p.RecipientCity,
		RecipientZipcode: p.RecipientZipcode,
		RecipientCountry: p.RecipientCountry,
		Currency:         p.Currency,
		OrderAmount:      p.OrderAmount,
		NetAmount:        p.NetAmount,
		TaxAmount:        p.TaxAmount,
		TaxRate:          p.TaxRate,
		Recipient:        recipient,
		Order:            p.Order,
		OrderContact:     p.OrderContact,
		Comment:          p.Comment,
		DeliveryDate:     p.DeliveryDate,
		DateCreated:      p.DateCreated,
		ExtraData:        p.ExtraData,
	}

}

// InitPurchaseOrderInput initialize the model based on the received parameters from the rest api call
func (p *PurchaseOrder) InitPurchaseOrderInput(payload *clientpurchaseorderpb.PurchaseOrderCreatePayload, self string) error {
	err := p.initPurchaseOrderFromData(payload.Data)
	if err != nil {
		return err
	}

	collaborators := append([]string{self}, payload.Collaborators...)
	cd, err := documents.NewCoreDocumentWithCollaborators(collaborators, compactPrefix())
	if err != nil {
		return errors.New("failed to init core document: %v", err)
	}

	p.CoreDocument = cd
	return nil
}

// initPurchaseOrderFromData initialises purchase order from purchaseOrderData
func (p *PurchaseOrder) initPurchaseOrderFromData(data *clientpurchaseorderpb.PurchaseOrderData) error {
	p.Status = data.PoStatus
	p.PoNumber = data.PoNumber
	p.OrderName = data.OrderName
	p.OrderStreet = data.OrderStreet
	p.OrderCity = data.OrderCity
	p.OrderZipcode = data.OrderZipcode
	p.OrderCountry = data.OrderCountry
	p.RecipientName = data.RecipientName
	p.RecipientStreet = data.RecipientStreet
	p.RecipientCity = data.RecipientCity
	p.RecipientZipcode = data.RecipientZipcode
	p.RecipientCountry = data.RecipientCountry
	p.Currency = data.Currency
	p.OrderAmount = data.OrderAmount
	p.NetAmount = data.NetAmount
	p.TaxAmount = data.TaxAmount
	p.TaxRate = data.TaxRate

	if data.Order != "" {
		order, err := hexutil.Decode(data.Order)
		if err != nil {
			return centerrors.Wrap(err, "failed to decode order")
		}

		p.Order = order
	}

	p.OrderContact = data.OrderContact
	p.Comment = data.Comment
	p.DeliveryDate = data.DeliveryDate
	p.DateCreated = data.DateCreated

	if data.Recipient != "" {
		if recipient, err := identity.NewDIDFromString(data.Recipient); err == nil {
			p.Recipient = &recipient
		}
	}

	if data.ExtraData != "" {
		ed, err := hexutil.Decode(data.ExtraData)
		if err != nil {
			return centerrors.Wrap(err, "failed to decode extra data")
		}

		p.ExtraData = ed
	}

	return nil
}

// loadFromP2PProtobuf loads the purcase order from centrifuge protobuf purchase order data
func (p *PurchaseOrder) loadFromP2PProtobuf(data *purchaseorderpb.PurchaseOrderData) {
	p.Status = data.PoStatus
	p.PoNumber = data.PoNumber
	p.OrderName = data.OrderName
	p.OrderStreet = data.OrderStreet
	p.OrderCity = data.OrderCity
	p.OrderZipcode = data.OrderZipcode
	p.OrderCountry = data.OrderCountry
	p.RecipientName = data.RecipientName
	p.RecipientStreet = data.RecipientStreet
	p.RecipientCity = data.RecipientCity
	p.RecipientZipcode = data.RecipientZipcode
	p.RecipientCountry = data.RecipientCountry
	p.Currency = data.Currency
	p.OrderAmount = data.OrderAmount
	p.NetAmount = data.NetAmount
	p.TaxAmount = data.TaxAmount
	p.TaxRate = data.TaxRate
	p.Order = data.Order
	p.OrderContact = data.OrderContact
	p.Comment = data.Comment
	p.DeliveryDate = data.DeliveryDate
	p.DateCreated = data.DateCreated
	p.ExtraData = data.ExtraData

	if data.Recipient != nil {
		recipient := identity.NewDIDFromBytes(data.Recipient)
		p.Recipient = &recipient
	}
}

// getPurchaseOrderSalts returns the purchase oder salts. Initialises if not present
func (p *PurchaseOrder) getPurchaseOrderSalts(purchaseOrderData *purchaseorderpb.PurchaseOrderData) (*proofs.Salts, error) {
	if p.PurchaseOrderSalts == nil {
		poSalts, err := documents.GenerateNewSalts(purchaseOrderData, prefix, compactPrefix())
		if err != nil {
			return nil, errors.New("getPOSalts error %v", err)
		}
		p.PurchaseOrderSalts = poSalts
	}

	return p.PurchaseOrderSalts, nil
}

// PackCoreDocument packs the PurchaseOrder into a Core Document
func (p *PurchaseOrder) PackCoreDocument() (cd coredocumentpb.CoreDocument, err error) {
	poData := p.createP2PProtobuf()
	data, err := proto.Marshal(poData)
	if err != nil {
		return cd, errors.New("failed to marshal po data: %v", err)
	}

	embedData := &any.Any{
		TypeUrl: p.DocumentType(),
		Value:   data,
	}

	salts, err := p.getPurchaseOrderSalts(poData)
	if err != nil {
		return cd, errors.New("failed to get po salts: %v", err)
	}

	return p.CoreDocument.PackCoreDocument(embedData, documents.ConvertToProtoSalts(salts)), nil
}

// UnpackCoreDocument unpacks the core document into PurchaseOrder
func (p *PurchaseOrder) UnpackCoreDocument(cd coredocumentpb.CoreDocument) error {
	if cd.EmbeddedData == nil ||
		cd.EmbeddedData.TypeUrl != p.DocumentType() {
		return errors.New("trying to convert document with incorrect schema")
	}

	poData := new(purchaseorderpb.PurchaseOrderData)
	err := proto.Unmarshal(cd.EmbeddedData.Value, poData)
	if err != nil {
		return err
	}

	p.loadFromP2PProtobuf(poData)
	if cd.EmbeddedDataSalts == nil {
		p.PurchaseOrderSalts, err = p.getPurchaseOrderSalts(poData)
		if err != nil {
			return err
		}
	} else {
		p.PurchaseOrderSalts = documents.ConvertToProofSalts(cd.EmbeddedDataSalts)
	}

	p.CoreDocument = documents.NewCoreDocumentFromProtobuf(cd)
	return err

}

// JSON marshals PurchaseOrder into a json bytes
func (p *PurchaseOrder) JSON() ([]byte, error) {
	return json.Marshal(p)
}

// FromJSON unmarshals the json bytes into PurchaseOrder
func (p *PurchaseOrder) FromJSON(jsonData []byte) error {
	return json.Unmarshal(jsonData, p)
}

// Type gives the PurchaseOrder type
func (p *PurchaseOrder) Type() reflect.Type {
	return reflect.TypeOf(p)
}

// CalculateDataRoot calculates the data root and sets the root to core document
func (p *PurchaseOrder) CalculateDataRoot() ([]byte, error) {
	t, err := p.getDocumentDataTree()
	if err != nil {
		return nil, errors.New("failed to get data tree: %v", err)
	}

	dr := t.RootHash()
	p.CoreDocument.SetDataRoot(dr)
	return dr, nil
}

// getDocumentDataTree creates precise-proofs data tree for the model
func (p *PurchaseOrder) getDocumentDataTree() (tree *proofs.DocumentTree, err error) {
	poProto := p.createP2PProtobuf()
	salts, err := p.getPurchaseOrderSalts(poProto)
	if err != nil {
		return nil, err
	}
	t := documents.NewDefaultTreeWithPrefix(salts, prefix, compactPrefix())
	err = t.AddLeavesFromDocument(poProto)
	if err != nil {
		return nil, errors.New("getDocumentDataTree error %v", err)
	}
	err = t.Generate()
	if err != nil {
		return nil, errors.New("getDocumentDataTree error %v", err)
	}
	return t, nil
}

// CreateProofs generates proofs for given fields.
func (p *PurchaseOrder) CreateProofs(fields []string) (proofs []*proofspb.Proof, err error) {
	tree, err := p.getDocumentDataTree()
	if err != nil {
		return nil, errors.New("createProofs error %v", err)
	}

	return p.CoreDocument.CreateProofs(p.DocumentType(), tree, fields)
}

// DocumentType returns the po document type.
func (*PurchaseOrder) DocumentType() string {
	return documenttypes.PurchaseOrderDataTypeUrl
}

// PrepareNewVersion prepares new version from the old invoice.
func (p *PurchaseOrder) PrepareNewVersion(old documents.Model, data *clientpurchaseorderpb.PurchaseOrderData, collaborators []string) error {
	err := p.initPurchaseOrderFromData(data)
	if err != nil {
		return err
	}

	oldCD := old.(*PurchaseOrder).CoreDocument
	p.CoreDocument, err = oldCD.PrepareNewVersion(collaborators, true, compactPrefix())
	if err != nil {
		return err
	}

	return nil
}

// AddNFT adds NFT to the Purchase Order.
func (p *PurchaseOrder) AddNFT(grantReadAccess bool, registry common.Address, tokenID []byte) error {
	cd, err := p.CoreDocument.AddNFT(grantReadAccess, registry, tokenID)
	if err != nil {
		return err
	}

	p.CoreDocument = cd
	return nil
}

// CalculateSigningRoot returns the signing root of the document.
// Calculates it if not generated yet.
func (p *PurchaseOrder) CalculateSigningRoot() ([]byte, error) {
	return p.CoreDocument.CalculateSigningRoot(p.DocumentType())
}

// CreateNFTProofs creates proofs specific to NFT minting.
func (p *PurchaseOrder) CreateNFTProofs(
	account identity.DID,
	registry common.Address,
	tokenID []byte,
	nftUniqueProof, readAccessProof bool) (proofs []*proofspb.Proof, err error) {
	return p.CoreDocument.CreateNFTProofs(
		p.DocumentType(),
		account, registry, tokenID, nftUniqueProof, readAccessProof)
}

// CollaboratorCanUpdate checks if the account can update the document.
func (p *PurchaseOrder) CollaboratorCanUpdate(updated documents.Model, collaborator identity.DID) error {
	newPo, ok := updated.(*PurchaseOrder)
	if !ok {
		return errors.NewTypedError(documents.ErrDocumentInvalidType, errors.New("expecting a purchase order but got %T", updated))
	}

	// check the core document changes
	err := p.CoreDocument.CollaboratorCanUpdate(newPo.CoreDocument, collaborator, p.DocumentType())
	if err != nil {
		return err
	}

	// check purchase order specific changes
	oldTree, err := p.getDocumentDataTree()
	if err != nil {
		return err
	}

	newTree, err := newPo.getDocumentDataTree()
	if err != nil {
		return err
	}

	rules := p.CoreDocument.TransitionRulesFor(collaborator)
	cf := documents.GetChangedFields(oldTree, newTree, proofs.DefaultSaltsLengthSuffix)
	return documents.ValidateTransitions(rules, cf)
}

// AddUpdateLog adds a log to the model to persist an update related meta data such as author
func (p *PurchaseOrder) AddUpdateLog(account identity.DID) (err error) {
	return p.CoreDocument.AddUpdateLog(account)
}

// Author is the author of the document version represented by the model
func (p *PurchaseOrder) Author() identity.DID {
	return p.CoreDocument.Author()
}

// Timestamp is the time of update in UTC of the document version represented by the model
func (p *PurchaseOrder) Timestamp() (time.Time, error) {
	return p.CoreDocument.Timestamp()
}
