package pkg

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yetanotherco/aligned_layer/metrics"

	"github.com/Layr-Labs/eigensdk-go/chainio/clients"
	sdkclients "github.com/Layr-Labs/eigensdk-go/chainio/clients"
	"github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/Layr-Labs/eigensdk-go/services/avsregistry"
	blsagg "github.com/Layr-Labs/eigensdk-go/services/bls_aggregation"
	oppubkeysserv "github.com/Layr-Labs/eigensdk-go/services/operatorpubkeys"
	eigentypes "github.com/Layr-Labs/eigensdk-go/types"
	"github.com/ethereum/go-ethereum/event"
	servicemanager "github.com/yetanotherco/aligned_layer/contracts/bindings/AlignedLayerServiceManager"
	"github.com/yetanotherco/aligned_layer/core/chainio"
	"github.com/yetanotherco/aligned_layer/core/config"
	"github.com/yetanotherco/aligned_layer/core/types"
	"github.com/yetanotherco/aligned_layer/core/utils"
)

// FIXME(marian): Read this from Aligned contract directly
const QUORUM_NUMBER = byte(0)
const QUORUM_THRESHOLD = byte(67)

// Aggregator stores TaskResponse for a task here
type TaskResponsesWithStatus struct {
	taskResponses       []types.SignedTaskResponse
	submittedToEthereum bool
}

type Aggregator struct {
	AggregatorConfig      *config.AggregatorConfig
	NewBatchChan          chan *servicemanager.ContractAlignedLayerServiceManagerNewBatch
	avsReader             *chainio.AvsReader
	avsSubscriber         *chainio.AvsSubscriber
	avsWriter             *chainio.AvsWriter
	taskSubscriber        event.Subscription
	blsAggregationService blsagg.BlsAggregationService

	// Using map here instead of slice to allow for easy lookup of tasks, when aggregator is restarting,
	// its easier to get the task from the map instead of filling the slice again
	tasks map[uint32][32]byte
	// Mutex to protect the tasks map
	tasksMutex *sync.Mutex

	OperatorTaskResponses map[[32]byte]*TaskResponsesWithStatus
	// Mutex to protect the taskResponses map
	taskResponsesMutex *sync.Mutex
	logger             logging.Logger

	// FIXME(marian): This is a hacky workaround to send some sensible index to the BLS aggregation service,
	// which needs a task index.
	taskCounter      uint32
	taskCounterMutex *sync.Mutex
	metricsReg       *prometheus.Registry
	metrics          *metrics.Metrics
}

func NewAggregator(aggregatorConfig config.AggregatorConfig) (*Aggregator, error) {
	newBatchChan := make(chan *servicemanager.ContractAlignedLayerServiceManagerNewBatch)

	avsReader, err := chainio.NewAvsReaderFromConfig(aggregatorConfig.BaseConfig, aggregatorConfig.EcdsaConfig)
	if err != nil {
		return nil, err
	}

	avsSubscriber, err := chainio.NewAvsSubscriberFromConfig(aggregatorConfig.BaseConfig)
	if err != nil {
		return nil, err
	}

	avsWriter, err := chainio.NewAvsWriterFromConfig(aggregatorConfig.BaseConfig, aggregatorConfig.EcdsaConfig)
	if err != nil {
		return nil, err
	}

	// tasks := make(map[uint32]servicemanager.AlignedLayerServiceManagerB)
	tasks := make(map[uint32][32]byte)
	operatorTaskResponses := make(map[[32]byte]*TaskResponsesWithStatus, 0)

	chainioConfig := sdkclients.BuildAllConfig{
		EthHttpUrl:                 aggregatorConfig.BaseConfig.EthRpcUrl,
		EthWsUrl:                   aggregatorConfig.BaseConfig.EthWsUrl,
		RegistryCoordinatorAddr:    aggregatorConfig.BaseConfig.AlignedLayerDeploymentConfig.AlignedLayerRegistryCoordinatorAddr.Hex(),
		OperatorStateRetrieverAddr: aggregatorConfig.BaseConfig.AlignedLayerDeploymentConfig.AlignedLayerOperatorStateRetrieverAddr.Hex(),
		AvsName:                    "AlignedLayer",
		PromMetricsIpPortAddress:   ":9090",
	}

	aggregatorPrivateKey := aggregatorConfig.EcdsaConfig.PrivateKey

	logger := aggregatorConfig.BaseConfig.Logger
	clients, err := clients.BuildAll(chainioConfig, aggregatorPrivateKey, logger)
	if err != nil {
		logger.Errorf("Cannot create sdk clients", "err", err)
		return nil, err
	}

	operatorPubkeysService := oppubkeysserv.NewOperatorPubkeysServiceInMemory(context.Background(), clients.AvsRegistryChainSubscriber, clients.AvsRegistryChainReader, logger)
	avsRegistryService := avsregistry.NewAvsRegistryServiceChainCaller(avsReader.AvsRegistryReader, operatorPubkeysService, logger)
	blsAggregationService := blsagg.NewBlsAggregatorService(avsRegistryService, logger)

	// Explicitly initializing this value just in case.
	taskCounter := uint32(0)

	// Metrics
	reg := prometheus.NewRegistry()
	aggregatorMetrics := metrics.NewMetrics(aggregatorConfig.Aggregator.MetricsIpPortAddress, reg, logger)

	aggregator := Aggregator{
		AggregatorConfig:      &aggregatorConfig,
		avsReader:             avsReader,
		avsSubscriber:         avsSubscriber,
		avsWriter:             avsWriter,
		NewBatchChan:          newBatchChan,
		tasks:                 tasks,
		tasksMutex:            &sync.Mutex{},
		OperatorTaskResponses: operatorTaskResponses,
		taskResponsesMutex:    &sync.Mutex{},
		blsAggregationService: blsAggregationService,
		logger:                logger,
		taskCounter:           taskCounter,
		taskCounterMutex:      &sync.Mutex{},
		metricsReg:            reg,
		metrics:               aggregatorMetrics,
	}

	return &aggregator, nil
}

func (agg *Aggregator) Start(ctx context.Context) error {
	agg.logger.Infof("Starting aggregator...")

	go func() {
		err := agg.ServeOperators()
		if err != nil {
			agg.logger.Fatal("Error listening for tasks", "err", err)
		}
	}()

	var metricsErrChan <-chan error
	if agg.AggregatorConfig.Aggregator.EnableMetrics {
		metricsErrChan = agg.metrics.Start(ctx, agg.metricsReg)
	} else {
		metricsErrChan = make(chan error, 1)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-metricsErrChan:
			agg.logger.Fatal("Metrics server failed", "err", err)
		case blsAggServiceResp := <-agg.blsAggregationService.GetResponseChannel():
			agg.logger.Info("Received response from BLS aggregation service", "blsAggServiceResp", blsAggServiceResp)
			agg.sendAggregatedResponseToContract(blsAggServiceResp)
			agg.metrics.IncAggregatedResponses()
		}
	}
}

func (agg *Aggregator) sendAggregatedResponseToContract(blsAggServiceResp blsagg.BlsAggregationServiceResponse) {
	if blsAggServiceResp.Err != nil {
		agg.logger.Error("BlsAggregationServiceResponse contains an error", "err", blsAggServiceResp.Err)
		return
	}

	nonSignerPubkeys := []servicemanager.BN254G1Point{}
	for _, nonSignerPubkey := range blsAggServiceResp.NonSignersPubkeysG1 {
		nonSignerPubkeys = append(nonSignerPubkeys, utils.ConvertToBN254G1Point(nonSignerPubkey))
	}
	quorumApks := []servicemanager.BN254G1Point{}
	for _, quorumApk := range blsAggServiceResp.QuorumApksG1 {
		quorumApks = append(quorumApks, utils.ConvertToBN254G1Point(quorumApk))
	}

	nonSignerStakesAndSignature := servicemanager.IBLSSignatureCheckerNonSignerStakesAndSignature{
		NonSignerPubkeys:             nonSignerPubkeys,
		QuorumApks:                   quorumApks,
		ApkG2:                        utils.ConvertToBN254G2Point(blsAggServiceResp.SignersApkG2),
		Sigma:                        utils.ConvertToBN254G1Point(blsAggServiceResp.SignersAggSigG1.G1Point),
		NonSignerQuorumBitmapIndices: blsAggServiceResp.NonSignerQuorumBitmapIndices,
		QuorumApkIndices:             blsAggServiceResp.QuorumApkIndices,
		TotalStakeIndices:            blsAggServiceResp.TotalStakeIndices,
		NonSignerStakeIndices:        blsAggServiceResp.NonSignerStakeIndices,
	}

	agg.logger.Info("Threshold reached. Sending aggregated response onchain.",
		"taskIndex", blsAggServiceResp.TaskIndex,
	)

	agg.tasksMutex.Lock()
	batchMerkleRoot := agg.tasks[blsAggServiceResp.TaskIndex]
	agg.tasksMutex.Unlock()

	_, err := agg.avsWriter.SendAggregatedResponse(context.Background(), batchMerkleRoot, nonSignerStakesAndSignature)
	if err != nil {
		agg.logger.Error("Aggregator failed to respond to task", "err", err)
	}
}

func (agg *Aggregator) AddNewTask(batchMerkleRoot [32]byte, taskCreatedBlock uint32) {
	agg.AggregatorConfig.BaseConfig.Logger.Info("Adding new task", "Batch merkle root", batchMerkleRoot)

	agg.taskCounterMutex.Lock()
	agg.taskCounter++
	agg.taskCounterMutex.Unlock()

	agg.tasksMutex.Lock()
	if _, ok := agg.tasks[agg.taskCounter]; ok {
		agg.logger.Warn("Task already exists", "taskIndex", agg.taskCounter)
		agg.tasksMutex.Unlock()
		return
	}
	agg.tasks[agg.taskCounter] = batchMerkleRoot
	agg.tasksMutex.Unlock()

	agg.taskResponsesMutex.Lock()
	agg.OperatorTaskResponses[batchMerkleRoot] = &TaskResponsesWithStatus{
		taskResponses:       make([]types.SignedTaskResponse, 0),
		submittedToEthereum: false,
	}
	agg.taskResponsesMutex.Unlock()

	quorumNums := eigentypes.QuorumNums{eigentypes.QuorumNum(QUORUM_NUMBER)}
	quorumThresholdPercentages := eigentypes.QuorumThresholdPercentages{eigentypes.QuorumThresholdPercentage(QUORUM_THRESHOLD)}

	// FIXME(marian): Hardcoded value of timeToExpiry to 100s. How should be get this value?
	err := agg.blsAggregationService.InitializeNewTask(agg.taskCounter, taskCreatedBlock, quorumNums, quorumThresholdPercentages, 100*time.Second)
	// FIXME(marian): When this errors, should we retry initializing new task? Logging fatal for now.
	if err != nil {
		agg.logger.Fatalf("BLS aggregation service error when initializing new task: %s", err)
	}
}
