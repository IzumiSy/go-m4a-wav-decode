package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	fdkaac "github.com/IzumiSy/go-fdkaac"
	mp4 "github.com/abema/go-mp4"

	"github.com/cryptix/wav"
)

var (
	inputFilePath string
	metaFilePath  string
	samplingRate  uint64
)

type metaInfo struct {
	FrameSizes []uint64 `json:"frame_sizes"`
	Offset     uint64   `json:"absolute_offset"`
}

func main() {
	flag.StringVar(&inputFilePath, "input", "", "input file")
	flag.StringVar(&metaFilePath, "meta", "", "meta file")
	flag.Uint64Var(&samplingRate, "sampleRate", 44100, "sampling rate in converting into wav")
	flag.Parse()

	if inputFilePath == "" {
		fmt.Println("input file is required")
		os.Exit(1)
	}

	file, err := os.Open(inputFilePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var (
		frameSizes *frameSizes
		mdatOffset uint64
	)

	// 外部から入力されるメタデータのJSONファイルがあればそれを使って
	// abema/go-mp4を用いたメタデータの読み取りはスキップする
	if metaFilePath != "" {
		metaFile, err := ioutil.ReadFile(metaFilePath)
		if err != nil {
			panic(err)
		}

		metaInfo := metaInfo{}
		if err := json.Unmarshal([]byte(metaFile), &metaInfo); err != nil {
			panic(err)
		}

		frameSizes, mdatOffset = newFrameSizesByMeta(metaInfo)
	} else {
		audioTrack, err := getAudioTrack(file)
		if err != nil {
			panic(err)
		}

		frameSizes, mdatOffset, err = newFrameSizes(audioTrack, file)
		if err != nil {
			panic(err)
		}
	}

	descriptor, err := getASCDescriptor(file)
	if err != nil {
		panic(err)
	} else if len(descriptor.Data) == 0 {
		panic(errors.New("no ASC available"))
	}
	fmt.Printf("ASC: 0x%X 0x%X\n", descriptor.Data[0], descriptor.Data[1])

	decoder := fdkaac.NewAacDecoder()
	if err := decoder.InitRaw([]byte{
		descriptor.Data[0],
		descriptor.Data[1],
	}); err != nil {
		panic(err)
	}
	defer decoder.Close()

	pcmReader, pcmWriter := io.Pipe()
	defer pcmReader.Close()

	var pcm []byte
	go func() {
		offset := uint32(mdatOffset)

		for {
			nextFrameSize := frameSizes.Next()
			if nextFrameSize == nil {
				break
			}

			// 計算されたフレームサイズのぶんだけmdatからデータを読み取る
			part := make([]byte, *nextFrameSize)
			readCount, err := io.NewSectionReader(file, int64(offset), int64(*nextFrameSize)).Read(part)
			if err == io.EOF {
				break
			} else if err != nil {
				panic(err)
			}

			// mdatから読み取ったデータに対してデコード処理を実行する
			if err = decoder.Decode(part[:readCount], pcmWriter); err != nil {
				panic(err)
			}

			offset += *nextFrameSize
			if len(pcm) == 0 {
				continue
			}
		}

		fmt.Println("AAC to PCM conversion finished")
		pcmWriter.Close()
	}()

	wavFile, err := os.Create("result.wav")
	if err != nil {
		panic(err)
	}
	defer wavFile.Close()

	// cryptix/wav supports only Monoral audio.
	meta := wav.File{
		Channels:        1,
		SampleRate:      uint32(samplingRate),
		SignificantBits: 32,
	}
	wavWriter, err := meta.NewWriter(wavFile)
	if err != nil {
		panic(err)
	}
	defer wavWriter.Close()
	if _, err := io.Copy(wavWriter, pcmReader); err != nil {
		panic(err)
	}

	fmt.Println("WAV file written out")
}

// stszセクションから取り出したraw aacのフレームサイズ情報を保持する構造体
type frameSizes struct {
	samples mp4.Samples
	index   int
}

// イテレーションしてフレームのサイズを返すメソッド
func (v *frameSizes) Next() *uint32 {
	if len(v.samples) <= v.index {
		return nil
	}

	v.index++
	return &v.samples[v.index-1].Size
}

// atom.Mp4Readerを用いてstszセクションのデータを読み取り、ヘッダをスキップしたデータ部をframeSizes構造体として抜き出す
func newFrameSizes(track *mp4.Track, reader io.ReadSeeker) (*frameSizes, uint64, error) {
	var mdatOffset uint64 = 0
	results, err := mp4.ExtractBox(reader, nil, mp4.BoxPath{mp4.BoxTypeMdat()})
	if err != nil {
		return nil, 0, err
	}

	// トラックIDに該当するindexにデータがなければおかしい
	if uint32(len(results)) < track.TrackID-1 {
		return nil, 0, errors.New("no stream data available on mdat")
	}
	stream := results[track.TrackID-1]

	// mdatのオフセットからさらにメタデータを含むヘッダサイズ分を飛ばして
	// 実際のメディアデータのバイナリが始まる位置をmdatOffsetとする
	mdatOffset = stream.Offset + stream.HeaderSize
	return &frameSizes{
		samples: track.Samples,
		index:   0,
	}, mdatOffset, nil
}

// 外部のファイルからframeSizesの構造体を生成する
func newFrameSizesByMeta(meta metaInfo) (*frameSizes, uint64) {
	samples := []*mp4.Sample{}
	for _, size := range meta.FrameSizes {
		samples = append(samples, &mp4.Sample{
			Size: uint32(size),
		})
	}

	return &frameSizes{
		samples: mp4.Samples(samples),
		index:   0,
	}, meta.Offset
}

// オーディオのstreamを取得する
func getAudioTrack(reader io.ReadSeeker) (*mp4.Track, error) {
	info, err := mp4.Probe(reader)
	if err != nil {
		panic(err)
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

	fmt.Printf("TrackID: %d\n", targetTrack.TrackID)
	fmt.Printf("Codec: %d\n", targetTrack.Codec)

	return targetTrack, nil
}

// esdsからASCの値を含むデスクリプタを取り出す
func getASCDescriptor(reader io.ReadSeeker) (*mp4.Descriptor, error) {
	var ascDescriptor *mp4.Descriptor

	if results, err := mp4.ExtractBoxWithPayload(reader, nil, mp4.BoxPath{
		mp4.BoxTypeMoov(),
		mp4.BoxTypeTrak(),
		mp4.BoxTypeMdia(),
		mp4.BoxTypeMinf(),
		mp4.BoxTypeStbl(),
		mp4.BoxTypeStsd(),
		mp4.BoxTypeMp4a(),
		mp4.BoxTypeEsds(),
	}); err != nil {
		return nil, err
	} else if len(results) != 1 {
		// esdsが複数あるわけないのであるとしたらなんかおかしい
		return nil, errors.New("too many esds")
	} else {
		esds := results[0].Payload.(*mp4.Esds)
		for _, descriptor := range esds.Descriptors {
			if descriptor.Tag == mp4.DecSpecificInfoTag {
				ascDescriptor = &descriptor
				break
			}
		}

		if ascDescriptor == nil {
			return nil, errors.New("no descriptor found")
		}
	}

	return ascDescriptor, nil
}
