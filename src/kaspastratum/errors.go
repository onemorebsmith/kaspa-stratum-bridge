package kaspastratum

type ErrorShortCodeT string

const (
	ErrFailedBlockFetch ErrorShortCodeT = "err_failed_block_fetch"
	ErrMissingJob       ErrorShortCodeT = "err_missing_job"
	ErrBadDataFromMiner ErrorShortCodeT = "err_bad_data_from_miner"
	ErrFailedSendWork   ErrorShortCodeT = "err_failed_sending_work"
	ErrFailedSetDiff    ErrorShortCodeT = "err_diff_set_failed"
	ErrDisconnected     ErrorShortCodeT = "err_worker_disconnected"
)
