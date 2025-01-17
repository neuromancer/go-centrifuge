// +build integration

package ethereum_test

import (
	"context"
	"testing"

	"github.com/centrifuge/go-centrifuge/testingutils/identity"

	"github.com/centrifuge/go-centrifuge/bootstrap"
	"github.com/centrifuge/go-centrifuge/ethereum"
	"github.com/centrifuge/go-centrifuge/identity"
	"github.com/centrifuge/go-centrifuge/queue"
	"github.com/centrifuge/go-centrifuge/transactions"
	"github.com/stretchr/testify/assert"
)

func enqueueJob(t *testing.T, txHash string) (transactions.Manager, identity.DID, transactions.TxID, chan bool) {
	queueSrv := ctx[bootstrap.BootstrappedQueueServer].(*queue.Server)
	txManager := ctx[transactions.BootstrappedService].(transactions.Manager)

	cid := testingidentity.GenerateRandomDID()
	tx, done, err := txManager.ExecuteWithinTX(context.Background(), cid, transactions.NilTxID(), "Check TX status", func(accountID identity.DID, txID transactions.TxID, txMan transactions.Manager, errChan chan<- error) {
		result, err := queueSrv.EnqueueJob(ethereum.EthTXStatusTaskName, map[string]interface{}{
			transactions.TxIDParam:           txID.String(),
			ethereum.TransactionAccountParam: cid.String(),
			ethereum.TransactionTxHashParam:  txHash,
		})
		if err != nil {
			errChan <- err
		}
		_, err = result.Get(txManager.GetDefaultTaskTimeout())
		if err != nil {
			errChan <- err
		}
		errChan <- nil
	})
	assert.NoError(t, err)

	return txManager, cid, tx, done
}

func TestTransactionStatusTask_successful(t *testing.T) {
	t.Parallel()
	txManager, cid, tx, result := enqueueJob(t, "0x1")

	r := <-result
	assert.True(t, r)
	trans, err := txManager.GetTransaction(cid, tx)
	assert.Nil(t, err, "a transaction should be returned")
	assert.Equal(t, string(transactions.Success), string(trans.Status), "transaction should be successful")
}

func TestTransactionStatusTask_failed(t *testing.T) {
	t.Parallel()
	txManager, cid, tx, result := enqueueJob(t, "0x2")

	r := <-result
	assert.True(t, r)
	trans, err := txManager.GetTransaction(cid, tx)
	assert.Nil(t, err, "a  centrifuge transaction should be  returned")
	assert.Equal(t, string(transactions.Failed), string(trans.Status), "transaction should fail")
}
