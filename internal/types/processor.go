package types

// TimelockEvent Timelock事件接口
type TimelockEvent interface {
	GetEventType() string
	GetContractAddress() string
	GetTxHash() string
	GetBlockNumber() uint64
}
