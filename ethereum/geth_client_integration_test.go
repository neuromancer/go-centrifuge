// +build integration

package ethereum_test

import (
	"os"
	"testing"
	"time"

	"github.com/centrifuge/go-centrifuge/identity/ideth"
	"github.com/centrifuge/go-centrifuge/transactions/txv1"

	"github.com/centrifuge/go-centrifuge/bootstrap"

	"github.com/centrifuge/go-centrifuge/bootstrap/bootstrappers/testlogging"
	"github.com/centrifuge/go-centrifuge/config"
	"github.com/centrifuge/go-centrifuge/config/configstore"
	"github.com/centrifuge/go-centrifuge/ethereum"
	"github.com/centrifuge/go-centrifuge/queue"
	"github.com/centrifuge/go-centrifuge/storage/leveldb"
	"github.com/centrifuge/go-centrifuge/testingutils/commons"
	"github.com/centrifuge/go-centrifuge/transactions"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var cfg config.Configuration
var ctx = map[string]interface{}{}

func registerMockedTransactionTask() {
	queueSrv := ctx[bootstrap.BootstrappedQueueServer].(*queue.Server)
	txManager := ctx[transactions.BootstrappedService].(transactions.Manager)

	mockClient := &testingcommons.MockEthClient{}

	// txHash: 0x1 -> successful
	mockClient.On("TransactionByHash", mock.Anything, common.HexToHash("0x1")).Return(&types.Transaction{}, false, nil).Once()
	mockClient.On("TransactionReceipt", mock.Anything, common.HexToHash("0x1")).Return(&types.Receipt{Status: 1}, nil).Once()

	// txHash: 0x2 -> fail
	mockClient.On("TransactionByHash", mock.Anything, common.HexToHash("0x2")).Return(&types.Transaction{}, false, nil).Once()
	mockClient.On("TransactionReceipt", mock.Anything, common.HexToHash("0x2")).Return(&types.Receipt{Status: 0}, nil).Once()

	// txHash: 0x3 -> pending
	mockClient.On("TransactionByHash", mock.Anything, common.HexToHash("0x3")).Return(&types.Transaction{}, true, nil).Maybe()

	ethTransTask := ethereum.NewTransactionStatusTask(200*time.Millisecond, txManager, mockClient.TransactionByHash, mockClient.TransactionReceipt, ethereum.DefaultWaitForTransactionMiningContext)
	queueSrv.RegisterTaskType(ethereum.EthTXStatusTaskName, ethTransTask)

}

func TestMain(m *testing.M) {
	var bootstappers = []bootstrap.TestBootstrapper{
		&testlogging.TestLoggingBootstrapper{},
		&config.Bootstrapper{},
		&leveldb.Bootstrapper{},
		txv1.Bootstrapper{},
		&queue.Bootstrapper{},
		ethereum.Bootstrapper{},
		&ideth.Bootstrapper{},
		&configstore.Bootstrapper{},
	}

	bootstrap.RunTestBootstrappers(bootstappers, ctx)
	cfg = ctx[bootstrap.BootstrappedConfig].(config.Configuration)

	queueStartBootstrap := &queue.Starter{}
	bootstappers = append(bootstappers, queueStartBootstrap)
	// register queue task
	registerMockedTransactionTask()
	//start queue
	queueStartBootstrap.TestBootstrap(ctx)

	result := m.Run()
	bootstrap.RunTestTeardown(bootstappers)
	os.Exit(result)
}

func TestGetConnection_returnsSameConnection(t *testing.T) {
	t.Parallel()
	howMany := 5
	confChannel := make(chan ethereum.Client, howMany)
	for ix := 0; ix < howMany; ix++ {
		go func() {
			confChannel <- ethereum.GetClient()
		}()
	}
	for ix := 0; ix < howMany; ix++ {
		multiThreadCreatedCon := <-confChannel
		assert.Equal(t, multiThreadCreatedCon, ethereum.GetClient(), "Should only return a single ethereum client")
	}
}

func TestNewGethClient(t *testing.T) {
	gc, err := ethereum.NewGethClient(cfg)
	assert.Nil(t, err)
	assert.NotNil(t, gc)
}
