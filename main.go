package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/IzumiSy/go-fdkaac/fdkaac"
	"github.com/alfg/mp4"
	"github.com/alfg/mp4/atom"
	"github.com/cryptix/wav"
)

func main() {
	m4aFile := "/home/izumisy/temp/sample.m4a"

	m4aData, err := os.Open(m4aFile)
	if err != nil {
		panic(err)
	}
	defer m4aData.Close()

	info, err := m4aData.Stat()
	if err != nil {
		panic(err)
	}

	v, err := mp4.OpenFromReader(m4aData, info.Size())
	if err != nil {
		panic(err)
	}

	// AAC LC/44100Hz/2channelsなASCの設定でfdk-aacのデコーダを初期化
	// (Ref: https://wiki.multimedia.cx/index.php/MPEG-4_Audio#Audio_Specific_Config)
	d := fdkaac.NewAacDecoder()
	if err := d.InitRaw([]byte{0x12, 0x10}); err != nil {
		panic(err)
	}
	defer d.Close()

	frameSizes, err := newFrameSizes(v)
	if err != nil {
		panic(err)
	}

	pcmReader, pcmWriter := io.Pipe()
	defer pcmReader.Close()

	const mdatOffset = 40

	var pcm []byte
	go func() {
		offset := int64(mdatOffset)

		for {
			nextFrameSize := frameSizes.Next()
			if nextFrameSize == nil {
				break
			}

			// 計算されたフレームサイズのぶんだけmdatからデータを読み取る
			part := make([]byte, *nextFrameSize)
			readCount, err := io.NewSectionReader(v.Mdat.Reader.Reader, offset, *nextFrameSize).Read(part)
			if err == io.EOF {
				break
			} else if err != nil {
				panic(err)
			}

			// mdatから読み取ったデータに対してデコード処理を実行する
			if pcm, err = d.Decode(part[:readCount]); err != nil {
				panic(err)
			}

			offset += *nextFrameSize
			if len(pcm) == 0 {
				continue
			}

			pcmWriter.Write(pcm)
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
		SampleRate:      44100,
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

	fmt.Println("done")
}

// stszセクションから取り出したraw aacのフレームサイズ情報を保持する構造体
type frameSizes struct {
	frameOffset uint
	size        uint
	buffer      []byte
}

// atom.Mp4Readerを用いてstszセクションのデータを読み取り、ヘッダをスキップしたデータ部をframeSizes構造体として抜き出す
func newFrameSizes(reader *atom.Mp4Reader) (*frameSizes, error) {
	const stszHeaderOffset = 12 + 8

	stsz := reader.Moov.Traks[0].Mdia.Minf.Stbl.Stsz
	stszBuffer := make([]byte, stsz.Size)
	if _, err := io.NewSectionReader(
		stsz.Reader.Reader,
		stsz.Start+int64(stszHeaderOffset),
		stsz.Size-int64(stszHeaderOffset),
	).Read(stszBuffer); err != nil {
		return nil, err
	}

	return &frameSizes{
		frameOffset: 0,
		buffer:      stszBuffer,
	}, nil
}

// stszのデータ部にはビッグエンディアンで4バイトごとのデータとして格納されている
// binary.BigEndian.Uint32で変換してint64に変換することで10進数データとしてフレームサイズが計算できる。
func (v *frameSizes) Next() *int64 {
	const uint32byteSize = 4

	if len(v.buffer) > int(v.frameOffset) {
		return nil
	}

	frameSize := int64(binary.BigEndian.Uint32(v.buffer[v.frameOffset : v.frameOffset+uint32byteSize]))
	v.frameOffset += uint32byteSize
	return &frameSize
}
