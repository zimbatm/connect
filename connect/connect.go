package connect

import (
	"errors"
	// "log"
	"fmt"
	"encoding/hex"
	"bytes"

	"github.com/oklog/ulid/v2"
)


const MaxMultihopLength = 8


// id for message to/from the platform
var ControlId = Id{}


// comparable
type TransferPath struct {
	SourceId Id
	DestinationId Id
	StreamId Id
}

func DestinationId(destinationId Id) TransferPath {
	return TransferPath{
		DestinationId: destinationId,
	}
}

func SourceId(sourceId Id) TransferPath {
	return TransferPath{
		SourceId: sourceId,
	}
}

func StreamId(streamId Id) TransferPath {
	return TransferPath{
		StreamId: streamId,
	}
}

func TransferPathFromBytes(
	sourceIdBytes []byte,
	destinationIdBytes []byte,
	streamIdBytes []byte,
) (path TransferPath, err error) {
	if sourceIdBytes != nil {
		path.SourceId, err = IdFromBytes(sourceIdBytes)
		if err != nil {
			return
		}
	}
	if destinationIdBytes != nil {
		path.DestinationId, err = IdFromBytes(destinationIdBytes)
		if err != nil {
			return
		}
	}
	if streamIdBytes != nil {
		path.StreamId, err = IdFromBytes(streamIdBytes)
		if err != nil {
			return
		}
	}
	return
}

func (self TransferPath) IsControlSource() {
	return self.StreamId == Id{} && self.SourceId == ControlId
}

func (self TransferPath) IsControlDestination() {
	return self.StreamId == Id{} && self.DestinationId == ControlId
}


// comparable
type Id [16]byte

func NewId() Id {
	return Id(ulid.Make())
}

func IdFromBytes(idBytes []byte) (Id, error) {
	if len(idBytes) != 16 {
		return Id{}, errors.New("Id must be 16 bytes")
	}
	return Id(idBytes), nil
}

func RequireIdFromBytes(idBytes []byte) Id {
	id, err := IdFromBytes(idBytes)
	if err != nil {
		panic(err)
	}
	return id
}

func ParseId(idStr string) (Id, error) {
	return parseUuid(idStr) 
}

func (self Id) Bytes() []byte {
	return self[0:16]
}

func (self Id) String() string {
	return encodeUuid(self)
}


func (self *Id) MarshalJSON() ([]byte, error) {
	var buf [16]byte
	copy(buf[0:16], self[0:16])
	var buff bytes.Buffer
	buff.WriteByte('"')
	buff.WriteString(encodeUuid(buf))
	buff.WriteByte('"')
	b := buff.Bytes()
	return b, nil
}

func (self *Id) UnmarshalJSON(src []byte) error {
	if len(src) != 38 {
		return fmt.Errorf("invalid length for UUID: %v", len(src))
	}
	buf, err := parseUuid(string(src[1 : len(src)-1]))
	if err != nil {
		return err
	}
	*self = buf
	return nil
}


func parseUuid(src string) (dst [16]byte, err error) {
	switch len(src) {
	case 36:
		src = src[0:8] + src[9:13] + src[14:18] + src[19:23] + src[24:]
	case 32:
		// dashes already stripped, assume valid
	default:
		// assume invalid.
		return dst, fmt.Errorf("cannot parse UUID %v", src)
	}

	buf, err := hex.DecodeString(src)
	if err != nil {
		return dst, err
	}

	copy(dst[:], buf)
	return dst, err
}


func encodeUuid(src [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", src[0:4], src[4:6], src[6:8], src[8:10], src[10:16])
}


// comparable
type MultiHopId struct {
	ids [MaxMultihopLength]Id
	len int
}

func NewMultiHopId(ids ... Id) (MultiHopId, error) {
	if MaxMultihopLength < len(ids) {
		return MultihopId{}, fmt.Errorf("Multihop length exceeds maximum: %d < %d", MaxMultihopLength, len(ids))
	}
	multihopId := MultihopId{
		len: len(ids),
	}
	for i, id := range ids {
		multihopId.ids[i] = id
	}
	return multihopId, nil
}

func MultiHopIdFromBytes(multiHopIdBytes [][]byte) (MultiHopId, error) {
	ids := []Id{}
	for i, idBytes := range multiHopIdBytes {
		if len(idBytes) != 16 {
			return Id{}, errors.New("Id must be 16 bytes")
		}
		ids = append(ids, Id(idBytes))
	}
	return NewMultiHopId(ids...), nil
}

func RequireMultiHopIdFromBytes(multiHopIdBytes [][]byte) MultiHopId {
	multiHopId, err := MultiHopIdFromBytes(multiHopIdBytes)
	if err != nil {
		panic(err)
	}
	return multiHopId
}

func (self MultiHopId) Len() int {
	return self.len
}

func (self MultiHopId) Ids() []Id {
	return self.ids[0:self.len]
}



// use this type when counting bytes
type ByteCount = int64

func kib(c ByteCount) ByteCount {
	return c * ByteCount(1024)
}

func mib(c ByteCount) ByteCount {
	return c * ByteCount(1024 * 1024)
}

func gib(c ByteCount) ByteCount {
	return c * ByteCount(1024 * 1024 * 1024)
}
