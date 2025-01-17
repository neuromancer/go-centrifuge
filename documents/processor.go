package documents

import (
	"context"
	"time"

	"github.com/centrifuge/centrifuge-protobufs/gen/go/coredocument"
	"github.com/centrifuge/centrifuge-protobufs/gen/go/p2p"
	"github.com/centrifuge/go-centrifuge/anchors"
	"github.com/centrifuge/go-centrifuge/contextutil"
	"github.com/centrifuge/go-centrifuge/errors"
	"github.com/centrifuge/go-centrifuge/identity"
	"github.com/centrifuge/go-centrifuge/utils"
)

// Config defines required methods required for the documents package.
type Config interface {
	GetNetworkID() uint32
	GetIdentityID() ([]byte, error)
	GetP2PConnectionTimeout() time.Duration
}

// Client defines methods that can be implemented by any type handling p2p communications.
type Client interface {

	// GetSignaturesForDocument gets the signatures for document
	GetSignaturesForDocument(ctx context.Context, model Model) ([]*coredocumentpb.Signature, []error, error)

	// after all signatures are collected the sender sends the document including the signatures
	SendAnchoredDocument(ctx context.Context, receiverID identity.DID, in *p2ppb.AnchorDocumentRequest) (*p2ppb.AnchorDocumentResponse, error)
}

// defaultProcessor implements AnchorProcessor interface
type defaultProcessor struct {
	identityService  identity.ServiceDID
	p2pClient        Client
	anchorRepository anchors.AnchorRepository
	config           Config
}

// DefaultProcessor returns the default implementation of CoreDocument AnchorProcessor
func DefaultProcessor(idService identity.ServiceDID, p2pClient Client, repository anchors.AnchorRepository, config Config) AnchorProcessor {
	return defaultProcessor{
		identityService:  idService,
		p2pClient:        p2pClient,
		anchorRepository: repository,
		config:           config,
	}
}

// Send sends the given defaultProcessor to the given recipient on the P2P layer
func (dp defaultProcessor) Send(ctx context.Context, cd coredocumentpb.CoreDocument, id identity.DID) (err error) {
	log.Infof("sending document %x to recipient %x", cd.DocumentIdentifier, id)
	ctx, cancel := context.WithTimeout(ctx, dp.config.GetP2PConnectionTimeout())
	defer cancel()

	resp, err := dp.p2pClient.SendAnchoredDocument(ctx, id, &p2ppb.AnchorDocumentRequest{Document: &cd})
	if err != nil || !resp.Accepted {
		return errors.New("failed to send document to the node: %v", err)
	}

	log.Infof("Sent document to %x\n", id)
	return nil
}

// PrepareForSignatureRequests gets the core document from the model, and adds the node's own signature
func (dp defaultProcessor) PrepareForSignatureRequests(ctx context.Context, model Model) error {
	self, err := contextutil.Account(ctx)
	if err != nil {
		return err
	}

	_, err = model.CalculateDataRoot()
	if err != nil {
		return err
	}

	id, err := self.GetIdentityID()
	if err != nil {
		return err
	}

	err = model.AddUpdateLog(identity.NewDIDFromBytes(id))
	if err != nil {
		return err
	}

	// calculate the signing root
	sr, err := model.CalculateSigningRoot()
	if err != nil {
		return errors.New("failed to calculate signing root: %v", err)
	}

	sig, err := self.SignMsg(sr)
	if err != nil {
		return err
	}

	model.AppendSignatures(sig)

	return nil
}

// RequestSignatures gets the core document from the model, validates pre signature requirements,
// collects signatures, and validates the signatures,
func (dp defaultProcessor) RequestSignatures(ctx context.Context, model Model) error {
	psv := SignatureValidator(dp.identityService)
	err := psv.Validate(nil, model)
	if err != nil {
		return errors.New("failed to validate model for signature request: %v", err)
	}

	// we ignore signature collection errors and anchor anyways
	signs, _, err := dp.p2pClient.GetSignaturesForDocument(ctx, model)
	if err != nil {
		return errors.New("failed to collect signatures from the collaborators: %v", err)
	}

	model.AppendSignatures(signs...)
	return nil
}

// PrepareForAnchoring validates the signatures and generates the document root
func (dp defaultProcessor) PrepareForAnchoring(model Model) error {
	psv := SignatureValidator(dp.identityService)
	err := psv.Validate(nil, model)
	if err != nil {
		return errors.New("failed to validate signatures: %v", err)
	}

	return nil
}

// PreAnchorDocument pre-commits a document
func (dp defaultProcessor) PreAnchorDocument(ctx context.Context, model Model) error {
	signingRoot, err := model.CalculateSigningRoot()
	if err != nil {
		return err
	}

	anchorID, err := anchors.ToAnchorID(model.CurrentVersion())
	if err != nil {
		return err
	}

	sRoot, err := anchors.ToDocumentRoot(signingRoot)
	if err != nil {
		return err
	}

	log.Infof("Pre-anchoring document with identifiers: [document: %#x, current: %#x, next: %#x], signingRoot: %#x", model.ID(), model.CurrentVersion(), model.NextVersion(), sRoot)
	done, err := dp.anchorRepository.PreCommitAnchor(ctx, anchorID, sRoot)

	isDone := <-done

	if !isDone {
		return errors.New("failed to pre-commit anchor: %v", err)
	}

	log.Infof("Pre-anchored document with identifiers: [document: %#x, current: %#x, next: %#x], signingRoot: %#x", model.ID(), model.CurrentVersion(), model.NextVersion(), sRoot)
	return nil
}

// AnchorDocument validates the model, and anchors the document
func (dp defaultProcessor) AnchorDocument(ctx context.Context, model Model) error {
	pav := PreAnchorValidator(dp.identityService)
	err := pav.Validate(nil, model)
	if err != nil {
		return errors.New("pre anchor validation failed: %v", err)
	}

	dr, err := model.CalculateDocumentRoot()
	if err != nil {
		return errors.New("failed to get document root: %v", err)
	}

	rootHash, err := anchors.ToDocumentRoot(dr)
	if err != nil {
		return errors.New("failed to get document root: %v", err)
	}

	anchorIDPreimage, err := anchors.ToAnchorID(model.CurrentVersionPreimage())
	if err != nil {
		return errors.New("failed to get anchor ID: %v", err)
	}

	signingRootProof, err := model.GetSignaturesRootHash()
	if err != nil {
		return errors.New("failed to get signing root proof: %v", err)
	}

	signingRootProofHashes, err := utils.ConvertProofForEthereum([][]byte{signingRootProof})
	if err != nil {
		return errors.New("failed to get signing root proof in ethereum format: %v", err)
	}

	log.Infof("Anchoring document with identifiers: [document: %#x, current: %#x, next: %#x], rootHash: %#x", model.ID(), model.CurrentVersion(), model.NextVersion(), dr)
	done, err := dp.anchorRepository.CommitAnchor(ctx, anchorIDPreimage, rootHash, signingRootProofHashes)

	isDone := <-done

	if !isDone {
		return errors.New("failed to commit anchor: %v", err)
	}

	log.Infof("Anchored document with identifiers: [document: %#x, current: %#x, next: %#x], rootHash: %#x", model.ID(), model.CurrentVersion(), model.NextVersion(), dr)
	return nil
}

// SendDocument does post anchor validations and sends the document to collaborators
func (dp defaultProcessor) SendDocument(ctx context.Context, model Model) error {
	av := PostAnchoredValidator(dp.identityService, dp.anchorRepository)
	err := av.Validate(nil, model)
	if err != nil {
		return errors.New("post anchor validations failed: %v", err)
	}

	selfDID, err := contextutil.AccountDID(ctx)
	if err != nil {
		return err
	}

	cs, err := model.GetSignerCollaborators(selfDID)
	if err != nil {
		return errors.New("get external collaborators failed: %v", err)
	}

	cd, err := model.PackCoreDocument()
	if err != nil {
		return errors.New("failed to pack core document: %v", err)
	}

	for _, c := range cs {
		erri := dp.Send(ctx, cd, c)
		if erri != nil {
			err = errors.AppendError(err, erri)
		}
	}

	return err
}
