package atom

import "encoding/binary"

// StszBox - Sample Sizes Framing Box
// Box Type: stsz
type StszBox struct {
	*Box
	Version     byte
	Flags       uint32
	SampleSize  uint32
	SampleCount uint32
}

func (b *StszBox) parse() error {
	data := b.ReadBoxData()
	b.Version = data[0]
	b.Flags = binary.BigEndian.Uint32(data[0:4])
	b.SampleSize = binary.BigEndian.Uint32(data[4:8])
	b.SampleCount = binary.BigEndian.Uint32(data[8:12])

	return nil
}
