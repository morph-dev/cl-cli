package agent

import (
	"fmt"
	"math/rand"

	"github.com/ethereum/go-ethereum/log"
)

var (
	requestTypeCal   = 0
	requestTypeChunk = 1
)

type request struct {
	requestType int
	chunkIndex  int
}

// Creates "cal" and "chunk" requests for chunks in range 0..count
//
// The created requests are randomized, but certain properties are guaranteed:
// - every cal and chunk will appear exactly once
// - every chunk will be preceded by cals of same or lower index
func randomizeChunkRequests(count int) []*request {
	requests := make([]*request, 0, 2*count)
	for i := range count {
		requests = append(requests, &request{
			requestType: requestTypeCal,
			chunkIndex:  i,
		})
		requests = append(requests, &request{
			requestType: requestTypeChunk,
			chunkIndex:  i,
		})
	}

	rand.Shuffle(len(requests), func(i, j int) {
		requests[i], requests[j] = requests[j], requests[i]
	})

	// Make sure that each "chunk" request is preceded by all "cal" requests of smaller or equal chunk index
	for i := 0; i < len(requests); i++ {
		req := requests[i]
		if req.requestType != requestTypeChunk {
			continue
		}

		requiredCals := 0
		for _, r := range requests[:i] {
			if r.requestType == requestTypeCal && r.chunkIndex <= req.chunkIndex {
				requiredCals++
			}
		}

		j := i
		for requiredCals <= req.chunkIndex {
			requests[j] = requests[j+1]
			if requests[j].requestType == requestTypeCal && requests[j].chunkIndex <= req.chunkIndex {
				requiredCals++
			}
			j++
		}
		requests[j] = req

		if j != i {
			// There were some changes, do a step back
			i--
		}
	}

	log.Debug("Requests", "count", count, "order", requests)
	return requests
}
func (r *request) String() string {
	var typeStr string
	switch r.requestType {
	case requestTypeCal:
		typeStr = "Cal"
	case requestTypeChunk:
		typeStr = "Chunk"
	}
	return fmt.Sprintf("r%s%d", typeStr, r.chunkIndex)
}
