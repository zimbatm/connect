package grouping

import (
	"fmt"
	"os"
	"unsafe"

	"bringyour.com/protocol"

	"google.golang.org/protobuf/proto"
)

type SessionID string

// Compare returns -1 if s < other, 0 if s == other, and 1 if s > other.
func (s SessionID) Compare(other SessionID) int {
	switch {
	case s < other:
		return -1
	case s > other:
		return 1
	default:
		return 0
	}
}

type CoOccurrenceData map[uint64]map[uint64]uint64

// used to precompute distances for clustering
type CoOccurrence struct {
	data      *CoOccurrenceData
	idMapping *map[SessionID]uint64 // map from session id to internal cooccurrence id (0 is never used as id)
	nextId    uint64
}

func NewCoOccurrence(cmapData *CoOccurrenceData) *CoOccurrence {
	if cmapData == nil {
		_cmapData := make(CoOccurrenceData, 0)
		cmapData = &_cmapData
	}
	idMap := make(map[SessionID]uint64, 0)
	return &CoOccurrence{
		data:      cmapData,
		idMapping: &idMap,
		nextId:    1, // 0 is never used as id
	}
}

// gets the internal id of the provided session id
// if the session id is not in the mapping, it is added as the next available id
func (c *CoOccurrence) getInternalId(sid SessionID) uint64 {
	cid, ok := (*c.idMapping)[sid]
	if !ok {
		cid = c.nextId
		(*c.idMapping)[sid] = cid
		c.nextId++
	}
	return cid
}

func (c *CoOccurrence) GetInternalMapping() *map[SessionID]uint64 {
	return c.idMapping
}

func (c *CoOccurrence) SetOuterKey(sid SessionID) {
	cid := c.getInternalId(sid)
	(*c.data)[cid] = make(map[uint64]uint64, 0)
}

func (c *CoOccurrence) CalcAndSet(ov1 Overlap, ov2 Overlap) {
	sid1 := ov1.SID()
	sid2 := ov2.SID()

	if sid1.Compare(sid2) == 0 { // sid1 == sid2
		return // no overlap with itself
	}

	totalOverlap := ov1.Overlap(ov2)
	if totalOverlap == 0 {
		return // do not record 0 overlap
	}

	cid1 := c.getInternalId(sid1)
	cid2 := c.getInternalId(sid2)

	switch sid1.Compare(sid2) {
	case -1: // sid1 < sid2
		if _, ok := (*c.data)[cid1]; !ok {
			(*c.data)[cid1] = make(map[uint64]uint64, 0)
		}
		(*c.data)[cid1][cid2] = totalOverlap
	case 1: // sid1 > sid2
		if _, ok := (*c.data)[cid2]; !ok {
			(*c.data)[cid2] = make(map[uint64]uint64, 0)
		}
		(*c.data)[cid2][cid1] = totalOverlap
	}
}

func (c *CoOccurrence) Get(sid1 SessionID, sid2 SessionID) uint64 {
	cid1 := c.getInternalId(sid1)
	cid2 := c.getInternalId(sid2)

	// if value doesn't exist then 0 value is returned (which is desired)
	if sid1.Compare(sid2) < 0 {
		return (*c.data)[cid1][cid2]
	}
	return (*c.data)[cid2][cid1]
}

func (c *CoOccurrence) GetData() *CoOccurrenceData {
	return c.data
}

func (c *CoOccurrence) GetNonZeroData() *[]uint64 {
	tuples := make([]uint64, 0)
	for _, innerMap := range *c.data {
		for _, overlap := range innerMap {
			if overlap != 0 {
				tuples = append(tuples, overlap)
			}
		}
	}
	return &tuples
}

func (c *CoOccurrence) SaveData(dataPath string) error {
	coocData := make([]*protocol.CoocOuter, 0)

	for outerCid, coocInner := range *c.data {
		outer := &protocol.CoocOuter{
			Cid: outerCid,
		}

		for innerCid, overlap := range coocInner {
			outer.CoocInner = append(outer.CoocInner, &protocol.CoocInner{
				Cid:     innerCid,
				Overlap: overlap,
			})
		}

		coocData = append(coocData, outer)
	}

	mappingData := make([]*protocol.CoocSid, 0)
	for sid, cid := range *c.idMapping {
		mappingData = append(mappingData, &protocol.CoocSid{
			Sid: string(sid),
			Cid: cid,
		})
	}

	dataToSave := &protocol.CooccurrenceData{
		CoocOuter: coocData,
		CoocSid:   mappingData,
	}

	out, err := proto.Marshal(dataToSave)
	if err != nil {
		return err
	}

	return os.WriteFile(dataPath, out, 0644)
}

func (c *CoOccurrence) LoadData(dataPath string) error {
	data, err := os.ReadFile(dataPath)
	if err != nil {
		return fmt.Errorf("could not read file: %w", err)
	}

	coocData := &protocol.CooccurrenceData{}
	if err := proto.Unmarshal(data, coocData); err != nil {
		return fmt.Errorf("could not unmarshal data: %w", err)
	}

	result := make(CoOccurrenceData, 0)
	for _, outer := range coocData.CoocOuter {
		innerMap := make(map[uint64]uint64)
		for _, inner := range outer.CoocInner {
			innerMap[inner.Cid] = inner.Overlap
		}
		result[outer.Cid] = innerMap
	}
	c.data = &result

	mapping := make(map[SessionID]uint64)
	for _, sid := range coocData.CoocSid {
		mapping[SessionID(sid.Sid)] = sid.Cid
	}
	c.idMapping = &mapping

	return nil
}

// MemorySize returns the size of the CoOccurrence in memory in bytes
// the data size of the full matrix for N sessionIDs is:
// s_key * N + (s_key + s_val)(N-1)N where s_key is the size of the key and s_val is the size of the value
// if keys are vals are uint64 (8 bytes) then we get 8N + 16(N-1)N ~ 16N^2 bytes
// if the matrix is not full then we can substitude (N-1) with the expected number of non-zero overlaps per sessionID
func (c *CoOccurrence) MemorySize() (uint64, uint64) {
	if c == nil {
		return 0, 0
	}

	dataSize := uint64(0)
	if c.data != nil {
		// calculate size of the CoOccurrenceData map
		for outerKey, innerMap := range *c.data {
			dataSize += uint64(unsafe.Sizeof(outerKey))
			for innerKey, value := range innerMap {
				dataSize += uint64(unsafe.Sizeof(innerKey))
				dataSize += uint64(unsafe.Sizeof(value))
			}
		}
	}

	idMapSize := uint64(0)
	if c.idMapping != nil {
		for id, value := range *c.idMapping {
			idMapSize += uint64(len(id))
			idMapSize += uint64(unsafe.Sizeof(value))
		}
	}

	return dataSize, idMapSize
}
