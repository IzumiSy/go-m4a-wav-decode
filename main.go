package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/IzumiSy/go-fdkaac/fdkaac"
	"github.com/alfg/mp4"
	"github.com/cryptix/wav"
)

var (
	aacData *os.File
	part    []byte
	err     error
	count   int
	pcm     []byte
)

func main() {
	m4aFile := "/home/izumisy/temp/m4asample.m4a"

	aacData, err = os.Open(m4aFile)
	if err != nil {
		log.Fatal(err)
	}
	defer aacData.Close()
	log.Println("Open aac file ", m4aFile)

	info, err := aacData.Stat()
	if err != nil {
		panic(err)
	}

	v, err := mp4.OpenFromReader(aacData, info.Size())
	if err != nil {
		panic(err)
	}

	d := fdkaac.NewAacDecoder()
	if err := d.InitRaw([]byte{0x12, 0x10}); err != nil {
		log.Fatal("init decoder failed: ", err)
		return
	}
	defer d.Close()

	const mdatOffset = 40
	const stszHeaderOffset = 12 + 8

	stsz := v.Moov.Traks[0].Mdia.Minf.Stbl.Stsz
	stszBuffer := make([]byte, stsz.Size)
	if _, err := io.NewSectionReader(
		stsz.Reader.Reader,
		stsz.Start+int64(stszHeaderOffset),
		stsz.Size-int64(stszHeaderOffset),
	).Read(stszBuffer); err != nil {
		panic(err)
	}

	pcmReader, pcmWriter := io.Pipe()
	defer pcmReader.Close()

	go func() {
		offset := int64(mdatOffset)
		for frameCount := 0; frameCount < len(stszBuffer); frameCount += 4 {
			frameSize := int64(binary.BigEndian.Uint32(stszBuffer[frameCount : frameCount+4]))

			part := make([]byte, frameSize)
			readCount, err := io.NewSectionReader(v.Mdat.Reader.Reader, offset, frameSize).Read(part)
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatal("read failed: ", err)
			}

			if pcm, err = d.Decode(part[:readCount]); err != nil {
				log.Fatal("decode failed: ", err)
			}

			offset += frameSize
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
		log.Fatal(err)
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
		log.Fatal(err)
	}
	defer wavWriter.Close()
	if _, err := io.Copy(wavWriter, pcmReader); err != nil {
		log.Fatal(err)
	}

	fmt.Println("done")
}
