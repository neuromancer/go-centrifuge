package ideth

import (
	"context"

	"github.com/centrifuge/go-centrifuge/contextutil"
	"github.com/centrifuge/go-centrifuge/errors"
	"github.com/centrifuge/go-centrifuge/ethereum"
	id "github.com/centrifuge/go-centrifuge/identity"
	"github.com/centrifuge/go-centrifuge/queue"
	"github.com/centrifuge/go-centrifuge/transactions"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("identity")

const identityCreatedEventName = "IdentityCreated(address)"

type factory struct {
	factoryAddress  common.Address
	factoryContract *FactoryContract
	client          ethereum.Client
	txManager       transactions.Manager
	queue           *queue.Server
}

// NewFactory returns a new identity factory service
func NewFactory(factoryContract *FactoryContract, client ethereum.Client, txManager transactions.Manager, queue *queue.Server, factoryAddress common.Address) id.Factory {
	return &factory{factoryAddress: factoryAddress, factoryContract: factoryContract, client: client, txManager: txManager, queue: queue}
}

func (s *factory) getNonceAt(ctx context.Context, address common.Address) (uint64, error) {
	// TODO: add blockNumber of the transaction which created the contract
	return s.client.GetEthClient().NonceAt(ctx, s.factoryAddress, nil)
}

// CalculateCreatedAddress calculates the Ethereum address based on address and nonce
func CalculateCreatedAddress(address common.Address, nonce uint64) common.Address {
	// How is a Ethereum address calculated:
	// See https://ethereum.stackexchange.com/questions/760/how-is-the-address-of-an-ethereum-contract-computed
	return crypto.CreateAddress(address, nonce)
}

func (s *factory) createIdentityTX(opts *bind.TransactOpts) func(accountID id.DID, txID transactions.TxID, txMan transactions.Manager, errOut chan<- error) {
	return func(accountID id.DID, txID transactions.TxID, txMan transactions.Manager, errOut chan<- error) {
		ethTX, err := s.client.SubmitTransactionWithRetries(s.factoryContract.CreateIdentity, opts)
		if err != nil {
			errOut <- err
			log.Infof("Failed to send identity for creation [txHash: %s] : %v", ethTX.Hash(), err)
			return
		}

		log.Infof("Sent off identity creation Ethereum transaction hash [%x] and Nonce [%v] and Check [%v]", ethTX.Hash(), ethTX.Nonce(), ethTX.CheckNonce())
		log.Infof("Transfer pending: 0x%x\n", ethTX.Hash())

		res, err := ethereum.QueueEthTXStatusTaskWithValue(accountID, txID, ethTX.Hash(), s.queue, &transactions.TXValue{Key: identityCreatedEventName, KeyIdx: 0})
		if err != nil {
			errOut <- err
			return
		}

		_, err = res.Get(txMan.GetDefaultTaskTimeout())
		if err != nil {
			errOut <- err
			return
		}
		errOut <- nil
	}

}

func (s *factory) CalculateIdentityAddress(ctx context.Context) (*common.Address, error) {
	nonce, err := s.getNonceAt(ctx, s.factoryAddress)
	if err != nil {
		return nil, err
	}

	identityAddress := CalculateCreatedAddress(s.factoryAddress, nonce)
	log.Infof("Calculated Address of the identity contract: 0x%x\n", identityAddress)
	return &identityAddress, nil
}

func isIdentityContract(identityAddress common.Address, client ethereum.Client) error {
	contractCode, err := client.GetEthClient().CodeAt(context.Background(), identityAddress, nil)
	if err != nil {
		return err
	}

	if len(contractCode) == 0 {
		return errors.New("bytecode for deployed identity contract %s not correct", identityAddress.String())
	}

	return nil

}

func (s *factory) IdentityExists(did *id.DID) (exists bool, err error) {
	opts, _ := s.client.GetGethCallOpts(false)
	valid, err := s.factoryContract.CreatedIdentity(opts, did.ToAddress())
	if err != nil {
		return false, err
	}
	return valid, nil
}

func (s *factory) CreateIdentity(ctx context.Context) (did *id.DID, err error) {
	tc, err := contextutil.Account(ctx)
	if err != nil {
		return nil, err
	}

	opts, err := s.client.GetTxOpts(tc.GetEthereumDefaultAccountName())
	if err != nil {
		log.Infof("Failed to get txOpts from Ethereum client: %v", err)
		return nil, err
	}

	calcIdentityAddress, err := s.CalculateIdentityAddress(ctx)
	if err != nil {
		return nil, err
	}

	createdDID := id.NewDID(*calcIdentityAddress)

	txID, done, err := s.txManager.ExecuteWithinTX(context.Background(), createdDID, transactions.NilTxID(), "Check TX for create identity status", s.createIdentityTX(opts))
	if err != nil {
		return nil, err
	}

	isDone := <-done
	// non async task
	if !isDone {
		return nil, errors.New("Create Identity TX failed: txID:%s", txID.String())
	}

	tx, err := s.txManager.GetTransaction(createdDID, txID)
	if err != nil {
		return nil, err
	}
	idCreated, ok := tx.Values[identityCreatedEventName]
	if !ok {
		return nil, errors.New("Couldn't find value for %s", identityCreatedEventName)
	}
	createdAddr := common.BytesToAddress(idCreated.Value)
	log.Infof("ID Created with address: %s", createdAddr.Hex())

	if calcIdentityAddress.Hex() != createdAddr.Hex() {
		log.Infof("[Recovered] Found race condition creating identity, calculatedDID[%s] vs createdDID[%s]", calcIdentityAddress.Hex(), createdAddr.Hex())
	}

	createdDID = id.NewDID(createdAddr)
	exists, err := s.IdentityExists(&createdDID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("Identity %s not found in factory registry", createdDID.String())
	}

	return &createdDID, nil
}
