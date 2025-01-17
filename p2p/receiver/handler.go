package receiver

import (
	"context"

	"github.com/centrifuge/centrifuge-protobufs/gen/go/p2p"
	"github.com/centrifuge/go-centrifuge/centerrors"
	"github.com/centrifuge/go-centrifuge/code"
	"github.com/centrifuge/go-centrifuge/config"
	"github.com/centrifuge/go-centrifuge/contextutil"
	"github.com/centrifuge/go-centrifuge/documents"
	"github.com/centrifuge/go-centrifuge/errors"
	"github.com/centrifuge/go-centrifuge/identity"
	"github.com/centrifuge/go-centrifuge/p2p/common"
	pb "github.com/centrifuge/go-centrifuge/protobufs/gen/go/protocol"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/libp2p/go-libp2p-protocol"
)

// Handler implements protocol message handlers
type Handler struct {
	config             config.Service
	handshakeValidator ValidatorGroup
	docSrv             documents.Service
	tokenRegistry      documents.TokenRegistry
	srvDID             identity.ServiceDID
}

// New returns an implementation of P2PServiceServer
func New(
	config config.Service,
	handshakeValidator ValidatorGroup,
	docSrv documents.Service,
	tokenRegistry documents.TokenRegistry,
	srvDID identity.ServiceDID) *Handler {
	return &Handler{
		config:             config,
		handshakeValidator: handshakeValidator,
		docSrv:             docSrv,
		tokenRegistry:      tokenRegistry,
		srvDID:             srvDID,
	}
}

// HandleInterceptor acts as main entry point for all message types, routes the request to the correct handler
func (srv *Handler) HandleInterceptor(ctx context.Context, peer peer.ID, protoc protocol.ID, msg *pb.P2PEnvelope) (*pb.P2PEnvelope, error) {
	if msg == nil {
		return convertToErrorEnvelop(errors.New("nil payload provided"))
	}
	envelope, err := p2pcommon.ResolveDataEnvelope(msg)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	DID, err := p2pcommon.ExtractDID(protoc)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	tc, err := srv.config.GetAccount(DID[:])
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	ctx, err = contextutil.New(ctx, tc)
	if err != nil {
		return convertToErrorEnvelop(err)
	}
	collaborator := identity.NewDIDFromBytes(envelope.Header.SenderId)
	err = srv.handshakeValidator.Validate(envelope.Header, &collaborator, &peer)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	switch p2pcommon.MessageTypeFromString(envelope.Header.Type) {
	case p2pcommon.MessageTypeRequestSignature:
		return srv.HandleRequestDocumentSignature(ctx, peer, protoc, envelope)
	case p2pcommon.MessageTypeSendAnchoredDoc:
		return srv.HandleSendAnchoredDocument(ctx, peer, protoc, envelope)
	case p2pcommon.MessageTypeGetDoc:
		return srv.HandleGetDocument(ctx, peer, protoc, envelope)
	default:
		return convertToErrorEnvelop(errors.New("MessageType [%s] not found", envelope.Header.Type))
	}

}

// HandleRequestDocumentSignature handles the RequestDocumentSignature message
func (srv *Handler) HandleRequestDocumentSignature(ctx context.Context, peer peer.ID, protoc protocol.ID, msg *p2ppb.Envelope) (*pb.P2PEnvelope, error) {
	req := new(p2ppb.SignatureRequest)
	err := proto.Unmarshal(msg.Body, req)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	collaborator := identity.NewDIDFromBytes(msg.Header.SenderId)
	res, err := srv.RequestDocumentSignature(ctx, req, collaborator)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	nc, err := srv.config.GetConfig()
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	p2pEnv, err := p2pcommon.PrepareP2PEnvelope(ctx, nc.GetNetworkID(), p2pcommon.MessageTypeRequestSignatureRep, res)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	return p2pEnv, nil
}

// RequestDocumentSignature signs the received document and returns the signature of the signingRoot
// Document signing root will be recalculated and verified
// Existing signatures on the document will be verified
// Document will be stored to the repository for state management
func (srv *Handler) RequestDocumentSignature(ctx context.Context, sigReq *p2ppb.SignatureRequest, collaborator identity.DID) (*p2ppb.SignatureResponse, error) {
	if sigReq == nil || sigReq.Document == nil {
		return nil, errors.New("nil document provided")
	}

	model, err := srv.docSrv.DeriveFromCoreDocument(*sigReq.Document)
	if err != nil {
		return nil, errors.New("failed to derive from core doc: %v", err)
	}

	signature, err := srv.docSrv.RequestDocumentSignature(ctx, model, collaborator)
	if err != nil {
		return nil, centerrors.New(code.Unknown, err.Error())
	}

	return &p2ppb.SignatureResponse{Signature: signature}, nil
}

// HandleSendAnchoredDocument handles the SendAnchoredDocument message
func (srv *Handler) HandleSendAnchoredDocument(ctx context.Context, peer peer.ID, protoc protocol.ID, msg *p2ppb.Envelope) (*pb.P2PEnvelope, error) {
	m := new(p2ppb.AnchorDocumentRequest)
	err := proto.Unmarshal(msg.Body, m)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	collaborator := identity.NewDIDFromBytes(msg.Header.SenderId)
	res, err := srv.SendAnchoredDocument(ctx, m, collaborator)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	nc, err := srv.config.GetConfig()
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	p2pEnv, err := p2pcommon.PrepareP2PEnvelope(ctx, nc.GetNetworkID(), p2pcommon.MessageTypeSendAnchoredDocRep, res)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	return p2pEnv, nil
}

// SendAnchoredDocument receives a new anchored document, validates and updates the document in DB
func (srv *Handler) SendAnchoredDocument(ctx context.Context, docReq *p2ppb.AnchorDocumentRequest, collaborator identity.DID) (*p2ppb.AnchorDocumentResponse, error) {
	if docReq == nil || docReq.Document == nil {
		return nil, errors.New("nil document provided")
	}

	model, err := srv.docSrv.DeriveFromCoreDocument(*docReq.Document)
	if err != nil {
		return nil, errors.New("failed to derive from core doc: %v", err)
	}

	err = srv.docSrv.ReceiveAnchoredDocument(ctx, model, collaborator)
	if err != nil {
		return nil, centerrors.New(code.Unknown, err.Error())
	}

	return &p2ppb.AnchorDocumentResponse{Accepted: true}, nil
}

// HandleGetDocument handles HandleGetDocument message
func (srv *Handler) HandleGetDocument(ctx context.Context, peer peer.ID, protoc protocol.ID, msg *p2ppb.Envelope) (*pb.P2PEnvelope, error) {
	m := new(p2ppb.GetDocumentRequest)
	err := proto.Unmarshal(msg.Body, m)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	requesterCentID := identity.NewDIDFromBytes(msg.Header.SenderId)

	res, err := srv.GetDocument(ctx, m, requesterCentID)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	nc, err := srv.config.GetConfig()
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	p2pEnv, err := p2pcommon.PrepareP2PEnvelope(ctx, nc.GetNetworkID(), p2pcommon.MessageTypeGetDocRep, res)
	if err != nil {
		return convertToErrorEnvelop(err)
	}

	return p2pEnv, nil
}

// GetDocument receives document identifier and retrieves the corresponding CoreDocument from the repository
func (srv *Handler) GetDocument(ctx context.Context, docReq *p2ppb.GetDocumentRequest, requester identity.DID) (*p2ppb.GetDocumentResponse, error) {
	model, err := srv.docSrv.GetCurrentVersion(ctx, docReq.DocumentIdentifier)
	if err != nil {
		return nil, err
	}

	if srv.validateDocumentAccess(ctx, docReq, model, requester) != nil {
		return nil, err
	}

	cd, err := model.PackCoreDocument()
	if err != nil {
		return nil, err
	}

	return &p2ppb.GetDocumentResponse{Document: &cd}, nil
}

// validateDocumentAccess validates the GetDocument request against the AccessType indicated in the request
func (srv *Handler) validateDocumentAccess(ctx context.Context, docReq *p2ppb.GetDocumentRequest, m documents.Model, peer identity.DID) error {
	// checks which access type is relevant for the request
	switch docReq.AccessType {
	case p2ppb.AccessType_ACCESS_TYPE_REQUESTER_VERIFICATION:
		if !m.AccountCanRead(peer) {
			return errors.New("requester does not have access")
		}
	case p2ppb.AccessType_ACCESS_TYPE_NFT_OWNER_VERIFICATION:
		registry := common.BytesToAddress(docReq.NftRegistryAddress)
		if m.NFTOwnerCanRead(srv.tokenRegistry, registry, docReq.NftTokenId, peer) != nil {
			return errors.New("requester does not have access")
		}
	case p2ppb.AccessType_ACCESS_TYPE_ACCESS_TOKEN_VERIFICATION:
		// check the document indicated by the delegating document identifier for the access token
		if docReq.AccessTokenRequest == nil {
			return errors.New("access token request is nil")
		}

		m, err := srv.docSrv.GetCurrentVersion(ctx, docReq.AccessTokenRequest.DelegatingDocumentIdentifier)
		if err != nil {
			return err
		}

		err = m.ATGranteeCanRead(ctx, srv.srvDID, docReq.AccessTokenRequest.AccessTokenId, docReq.DocumentIdentifier, peer)
		if err != nil {
			return err
		}
	default:
		return errors.New("invalid access type")
	}
	return nil
}

func convertToErrorEnvelop(err error) (*pb.P2PEnvelope, error) {
	errPb, ok := err.(proto.Message)
	if !ok {
		return nil, err
	}
	errBytes, err := proto.Marshal(errPb)
	if err != nil {
		return nil, err
	}

	envelope := &p2ppb.Envelope{
		Header: &p2ppb.Header{
			Type: p2pcommon.MessageTypeError.String(),
		},
		Body: errBytes,
	}

	marshalledOut, err := proto.Marshal(envelope)
	if err != nil {
		return nil, err
	}

	// an error for the client
	return &pb.P2PEnvelope{Body: marshalledOut}, nil
}
