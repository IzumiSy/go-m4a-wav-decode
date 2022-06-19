package mp4audio

import (
	"errors"
	"io"

	mp4 "github.com/abema/go-mp4"
	"github.com/sunfish-shogi/bufseekio"
)

type MP4Audio struct {
	reader     io.ReadSeeker
	track      *mp4.Track
	trackIndex int
}

func New(reader io.ReadSeeker) (*MP4Audio, error) {
	bufferedReader := bufseekio.NewReadSeeker(reader, 256*256, 4)
	audioTrack, err := getAudioTrack(bufferedReader)
	if err != nil {
		return nil, err
	}

	return &MP4Audio{
		reader:     reader,
		track:      audioTrack,
		trackIndex: int(audioTrack.TrackID) - 1,
	}, nil
}

// フレーム情報のイテレータを生成する
func (mp4audio *MP4Audio) Frames() (*FrameIterator, error) {
	stcoResults, err := mp4.ExtractBoxWithPayload(mp4audio.reader, nil, mp4.BoxPath{
		mp4.BoxTypeMoov(),
		mp4.BoxTypeTrak(),
		mp4.BoxTypeMdia(),
		mp4.BoxTypeMinf(),
		mp4.BoxTypeStbl(),
		mp4.BoxTypeStco(),
	})
	if err != nil {
		panic(err)
	}

	if len(stcoResults) < mp4audio.trackIndex {
		return nil, errors.New("no stco atom available for audio track")
	}
	stco := stcoResults[mp4audio.trackIndex].Payload.(*mp4.Stco)

	if len(stco.ChunkOffset) == 0 {
		return nil, errors.New("no audio chunk available")
	}

	stscResults, err := mp4.ExtractBoxWithPayload(mp4audio.reader, nil, mp4.BoxPath{
		mp4.BoxTypeMoov(),
		mp4.BoxTypeTrak(),
		mp4.BoxTypeMdia(),
		mp4.BoxTypeMinf(),
		mp4.BoxTypeStbl(),
		mp4.BoxTypeStsc(),
	})
	if err != nil {
		panic(err)
	}

	if len(stscResults) < mp4audio.trackIndex {
		return nil, errors.New("no stsc atom available for audio track")
	}
	stsc := stscResults[mp4audio.trackIndex].Payload.(*mp4.Stsc)

	firstEntry := stsc.Entries[0]
	return &FrameIterator{
		samples:           mp4audio.track.Samples,
		chunkOffsets:      stco.ChunkOffset,
		stscEntries:       stsc.Entries,
		sampleIndex:       0,
		chunkIndex:        0,
		chunkSamples:      firstEntry.SamplesPerChunk,
		chunkSampleIndex:  0,
		chunkSampleOffset: uint64(stco.ChunkOffset[0]),
	}, nil
}

type FrameIterator struct {
	samples           mp4.Samples
	chunkOffsets      []uint32
	stscEntries       []mp4.StscEntry
	sampleIndex       int
	chunkSamples      uint32
	chunkIndex        int
	chunkSampleIndex  int
	chunkSampleOffset uint64
}

type Frame struct {
	Offset uint64
	Size   uint64
}

// イテレーションしてフレームの情報を返すメソッド
func (v *FrameIterator) Next() *Frame {
	if len(v.samples) <= v.sampleIndex || len(v.chunkOffsets) <= v.chunkIndex {
		return nil
	}

	offset := v.chunkSampleOffset
	size := uint64(v.samples[v.sampleIndex].Size)

	// println(offset, v.chunkIndex, v.chunkSamples, v.chunkSampleIndex)

	if v.chunkSamples <= uint32(v.chunkSampleIndex) {
		v.chunkIndex++
		v.chunkSamples = v.stscEntries[v.chunkIndex].SamplesPerChunk
		v.chunkSampleIndex = 0
		v.chunkSampleOffset = uint64(v.chunkOffsets[v.chunkIndex])
	} else {
		v.chunkSampleIndex++
		v.chunkSampleOffset += uint64(v.samples[v.sampleIndex].Size)
		v.sampleIndex++
	}

	return &Frame{
		Offset: offset,
		Size:   size,
	}
}

// esdsからASCの値を含むデスクリプタを取り出す
func (mp4audio *MP4Audio) ASCDescriptor() (*mp4.Descriptor, error) {
	results, err := mp4.ExtractBoxWithPayload(mp4audio.reader, nil, mp4.BoxPath{
		mp4.BoxTypeMoov(),
		mp4.BoxTypeTrak(),
		mp4.BoxTypeMdia(),
		mp4.BoxTypeMinf(),
		mp4.BoxTypeStbl(),
		mp4.BoxTypeStsd(),
		mp4.BoxTypeMp4a(),
		mp4.BoxTypeEsds(),
	})
	if err != nil {
		return nil, err
	}

	// audioトラックがあるのにesdsが無いのはおかしい
	if len(results) == 0 {
		return nil, errors.New("no esds atom available")
	}
	esds := results[0].Payload.(*mp4.Esds)

	var ascDescriptor *mp4.Descriptor
	for _, descriptor := range esds.Descriptors {
		if descriptor.Tag == mp4.DecSpecificInfoTag {
			ascDescriptor = &descriptor
			break
		}
	}

	if ascDescriptor == nil {
		return nil, errors.New("no descriptor found")
	}

	return ascDescriptor, nil
}

// オーディオのstreamを取得する
func getAudioTrack(reader io.ReadSeeker) (*mp4.Track, error) {
	info, err := mp4.Probe(reader)
	if err != nil {
		return nil, err
	}

	var targetTrack *mp4.Track
	for _, track := range info.Tracks {
		if track.Codec == mp4.CodecMP4A {
			targetTrack = track
			break
		}
	}
	if targetTrack == nil {
		return nil, errors.New("no audio track available")
	}

	return targetTrack, nil
}
