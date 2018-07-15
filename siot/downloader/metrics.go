// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/ethereum/go-ethereum/metrics"
)

var (
	headerInMeter      = metrics.NewMeter("siot/downloader/headers/in")
	headerReqTimer     = metrics.NewTimer("siot/downloader/headers/req")
	headerDropMeter    = metrics.NewMeter("siot/downloader/headers/drop")
	headerTimeoutMeter = metrics.NewMeter("siot/downloader/headers/timeout")

	bodyInMeter      = metrics.NewMeter("siot/downloader/bodies/in")
	bodyReqTimer     = metrics.NewTimer("siot/downloader/bodies/req")
	bodyDropMeter    = metrics.NewMeter("siot/downloader/bodies/drop")
	bodyTimeoutMeter = metrics.NewMeter("siot/downloader/bodies/timeout")

	receiptInMeter      = metrics.NewMeter("siot/downloader/receipts/in")
	receiptReqTimer     = metrics.NewTimer("siot/downloader/receipts/req")
	receiptDropMeter    = metrics.NewMeter("siot/downloader/receipts/drop")
	receiptTimeoutMeter = metrics.NewMeter("siot/downloader/receipts/timeout")

	stateInMeter      = metrics.NewMeter("siot/downloader/states/in")
	stateReqTimer     = metrics.NewTimer("siot/downloader/states/req")
	stateDropMeter    = metrics.NewMeter("siot/downloader/states/drop")
	stateTimeoutMeter = metrics.NewMeter("siot/downloader/states/timeout")
)
