// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

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
