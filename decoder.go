package main

import (
	"flag"
	"fmt"
	"go-m4a-wav-decode/mp4audio"
	"io"
	"os"

	fdkaac "github.com/IzumiSy/go-fdkaac"
	"github.com/cryptix/wav"
)

var (
	inputFilePath string
	samplingRate  uint64
)

type metaInfo struct {
	FrameSizes []uint64 `json:"frame_sizes"`
	Offset     uint64   `json:"absolute_offset"`
}

func main() {
	flag.StringVar(&inputFilePath, "input", "", "input file")
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

	mp4Audio, err := mp4audio.New(file)
	if err != nil {
		panic(err)
	}

	ascDescriptor, err := mp4Audio.ASCDescriptor()
	if err != nil {
		panic(err)
	}

	decoder := fdkaac.NewAacDecoder()
	if err := decoder.InitRaw([]byte{
		ascDescriptor.Data[0],
		ascDescriptor.Data[1],
	}); err != nil {
		panic(err)
	}
	defer decoder.Close()

	pcmReader, pcmWriter := io.Pipe()
	defer pcmReader.Close()

	frameIterator, err := mp4Audio.Frames()
	if err != nil {
		panic(err)
	}

	var pcm []byte
	go func() {
		offset := uint32(mp4Audio.MdatOffset)

		for {
			nextFrame := frameIterator.Next()
			if nextFrame == nil {
				break
			}
			nextFrameSize := nextFrame.Size

			// 計算されたフレームサイズのぶんだけmdatからデータを読み取る
			part := make([]byte, nextFrameSize)
			readCount, err := io.NewSectionReader(file, int64(offset), int64(nextFrameSize)).Read(part)
			if err == io.EOF {
				break
			} else if err != nil {
				panic(err)
			}

			// mdatから読み取ったデータに対してデコード処理を実行する
			if err = decoder.Decode(part[:readCount], pcmWriter); err != nil {
				panic(err)
			}

			offset += nextFrameSize
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
