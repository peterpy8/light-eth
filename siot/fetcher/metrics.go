// Contains the metrics collected by the fetcher.

package fetcher

import (
	"github.com/ethereum/go-ethereum/metrics"
)

var (
	propAnnounceInMeter   = metrics.NewMeter("siot/fetcher/prop/announces/in")
	propAnnounceOutTimer  = metrics.NewTimer("siot/fetcher/prop/announces/out")
	propAnnounceDropMeter = metrics.NewMeter("siot/fetcher/prop/announces/drop")
	propAnnounceDOSMeter  = metrics.NewMeter("siot/fetcher/prop/announces/dos")

	propBroadcastInMeter   = metrics.NewMeter("siot/fetcher/prop/broadcasts/in")
	propBroadcastOutTimer  = metrics.NewTimer("siot/fetcher/prop/broadcasts/out")
	propBroadcastDropMeter = metrics.NewMeter("siot/fetcher/prop/broadcasts/drop")
	propBroadcastDOSMeter  = metrics.NewMeter("siot/fetcher/prop/broadcasts/dos")

	headerFetchMeter = metrics.NewMeter("siot/fetcher/fetch/headers")
	bodyFetchMeter   = metrics.NewMeter("siot/fetcher/fetch/bodies")

	headerFilterInMeter  = metrics.NewMeter("siot/fetcher/filter/headers/in")
	headerFilterOutMeter = metrics.NewMeter("siot/fetcher/filter/headers/out")
	bodyFilterInMeter    = metrics.NewMeter("siot/fetcher/filter/bodies/in")
	bodyFilterOutMeter   = metrics.NewMeter("siot/fetcher/filter/bodies/out")
)
